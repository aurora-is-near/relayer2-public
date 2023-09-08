package account

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aurora-is-near/near-api-go"
	"github.com/aurora-is-near/relayer2-base/endpoint"
	"github.com/aurora-is-near/relayer2-base/log"
	"github.com/aurora-is-near/relayer2-base/types/common"
	"github.com/aurora-is-near/relayer2-base/types/engine"
	"github.com/aurora-is-near/relayer2-base/utils"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
)

const (
	qCap                          = 100
	timeoutSeconds                = 60
	timeoutSeconds2               = 2 * timeoutSeconds
	enqueueWaitMilliseconds       = 100
	minNonceUpdateIntervalSeconds = 2
)

type TxnProcessor struct {
	mapper         TxnMapper
	config         *endpoint.EngineConfig
	logger         *log.Logger
	account        *near.Account
	ethNonceCache  map[string]*nonceCache
	nearNonceCache map[string]*nonceCache
	keys           []string
	retryMutex     []*sync.Mutex
	ordered        []*TxnQ[TxnReq]
	unordered      []*TxnQ[TxnReq]
	circuitBreaker []bool
}

type nonceCache struct {
	nonce      uint64
	updateTime time.Time
}

// NewTxnProcessor creates and starts transaction processor which is responsible for sending received Eth transaction to
// Near in a controlled fashion. Also see, TxnProcessor.Submit(*TxnReq)
func NewTxnProcessor(config *endpoint.EngineConfig, account *near.Account) (*TxnProcessor, error) {

	var mapper TxnMapper
	switch config.FunctionKeyMapper {
	case "CRC32":
		mapper = CRC32Mapper{}
	case "RoundRobin":
		mapper = RoundRobinMapper{}
	}

	txnProcessor := &TxnProcessor{
		mapper:         mapper,
		config:         config,
		account:        account,
		logger:         log.Log(),
		keys:           account.GetVerifiedAccessKeys(),
		ethNonceCache:  make(map[string]*nonceCache),
		nearNonceCache: make(map[string]*nonceCache),
		ordered:        make([]*TxnQ[TxnReq], 0),
		unordered:      make([]*TxnQ[TxnReq], 0),
		retryMutex:     make([]*sync.Mutex, 0),
		circuitBreaker: make([]bool, 0),
	}

	for range txnProcessor.keys {
		unorderedQ, err := NewTxnQ[TxnReq](qCap)
		if err != nil {
			return nil, err
		}
		txnProcessor.unordered = append(txnProcessor.unordered, unorderedQ)
		unorderedQ.StartWorker(txnProcessor.sort)

		orderedQ, err := NewTxnQ[TxnReq](qCap)
		if err != nil {
			return nil, err
		}
		txnProcessor.ordered = append(txnProcessor.ordered, orderedQ)
		orderedQ.StartWorker(txnProcessor.send)

		txnProcessor.retryMutex = append(txnProcessor.retryMutex, &sync.Mutex{})
		txnProcessor.circuitBreaker = append(txnProcessor.circuitBreaker, false)

	}

	return txnProcessor, nil
}

// NewTxnReq creates a new TxnReq from the raw transaction bytes. It also performs Sender Address, Gas Limit and Gas Price
// validations.
func (tp *TxnProcessor) NewTxnReq(rawTxn []byte) (*TxnReq, error) {

	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(rawTxn); err != nil {
		return nil, fmt.Errorf("failed to decode transaction, err: [%v]", err)
	}

	// check if provided sender address is valid
	addr, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract transaction sender, err: [%v]", err)
	}
	sender := common.BytesToAddress(addr.Bytes())

	// check if provided gas price is bigger than min gas price limit
	if tx.GasPrice().Cmp(tp.config.MinGasPrice) < 0 {
		return nil, fmt.Errorf("gas price too low")
	}

	// check if provided gas limit is bigger than min gas limit
	if tx.Gas() < tp.config.MinGasLimit {
		return nil, fmt.Errorf("intrinsic gas too low")
	}

	requestId := uuid.New()

	return &TxnReq{
		timestamp:      time.Now().Unix(),
		processorIndex: tp.mapper.Map(sender.Hex(), len(tp.keys)),
		requestId:      requestId,
		sender:         sender,
		nonce:          tx.Nonce(),
		rawTxn:         rawTxn,
		respChan:       make(chan *txnResp, 1),
		hash:           utils.CalculateKeccak256(rawTxn),
		retransmit:     false,
	}, nil
}

// Submit starts the transaction processing for TxnReq. It returns a pointer to TxnResp which is a handle to
// get actual transaction response. Also see, TxnResp.Get()
func (tp *TxnProcessor) Submit(req *TxnReq) *TxnResp {

	resp := &TxnResp{
		txnResp: &txnResp{
			status: txnStatus_Any,
		},
		received: false,
		done:     make(chan struct{}, 1),
	}

	// wait for response with timeout
	go func(req *TxnReq, resp *TxnResp) {
		for {
			select {
			case <-time.After(time.Second * timeoutSeconds * 3):
				tp.logger.Error().Msgf("Unexpected Txn timeout for eth nonce [%d]", req.nonce)
				resp.SetWithStatus(txnStatus_TimeOutError1)
			case r := <-req.respChan:
				resp.Set(r)
			}
			if !tp.eval(req, resp) {
				break
			}
			if !tp.unordered[req.processorIndex].TryEnqueue(req) {
				break
			}
			tp.logger.Info().Msgf("retrying txn [%s] - sender: [%s], eth nonce: [%d]", req.hash, req.sender.Hex(), req.nonce)
		}
		// notify response waiter if any, i.e.: sync requests
		resp.Notify()
	}(req, resp)

	tp.unordered[req.processorIndex].Enqueue(req)
	return resp
}

// Close gracefully stops TxnProcessor, i.e.: waits for all received transactions to be completed.
func (tp *TxnProcessor) Close() {

	for _, q := range tp.unordered {
		q.Close()
	}
	for _, q := range tp.ordered {
		q.Close()
	}
}

func (tp *TxnProcessor) sort(req *TxnReq) {

	tp.retryMutex[req.processorIndex].Lock()
	defer tp.retryMutex[req.processorIndex].Unlock()

	if req.Expired() {
		req.RespondWithStatus(txnStatus_TimeOutError3)
		return
	}

	if nc, ok := tp.ethNonceCache[req.sender.Hex()]; !ok {
		// if cache miss, then we assume the first received nonce is correct
		tp.ethNonceCache[req.sender.Hex()] = &nonceCache{
			nonce:      req.nonce + 1,
			updateTime: time.Now().Add(time.Second * time.Duration(-minNonceUpdateIntervalSeconds)),
		}

		tp.ordered[req.processorIndex].Enqueue(req)
		tp.logger.Info().Msgf("sort -> init txs: [%d]", req.nonce) // TODO: make the log DEBUG level
	} else {
		// if cache hit, compare with request's nonce
		if nc.nonce == req.nonce {
			// nonce match, push request to ordered Q
			nc.nonce += 1
			tp.ordered[req.processorIndex].Enqueue(req)
			tp.logger.Info().Msgf("sort -> ordering txs: [%d]", req.nonce) // TODO: make the log DEBUG level
		} else if req.retransmit { //if retransmit flag is set
			tp.ordered[req.processorIndex].Enqueue(req)
			tp.logger.Info().Msgf("sort -> retransmit txs: [%d]", req.nonce) // TODO: make the log DEBUG level
		} else if nc.nonce > req.nonce {
			// nonce is low, discard the request (nothing to do)
			req.RespondWithStatus(txnStatus_EthNonceTooLow)
			tp.logger.Info().Msgf("sort -> unordered txs: [%d]", req.nonce) // TODO: make the log DEBUG level
		} else {
			// non-blocking wait used to prevent excessive CPU usage
			time.AfterFunc(enqueueWaitMilliseconds*time.Millisecond, func() {
				// nonce is high, enqueue this request and wait for another with correct nonce
				if !tp.unordered[req.processorIndex].TryEnqueue(req) {
					req.RespondWithStatus(txnStatus_Exhausted)
				}
			})

			// trigger eth nonce cache update
			tp.updateEthNonceCache(req.sender)
		}
	}
}

func (tp *TxnProcessor) send(req *TxnReq) {
	if tp.circuitBreaker[req.processorIndex] {
		req.RespondWithStatus(txnStatus_Reorder)
		return
	}

	if req.Expired() {
		req.RespondWithStatus(txnStatus_TimeOutError3)
		return
	}

	key := tp.keys[req.processorIndex]
	nonce, err := tp.nextNearNonce(key)
	if err != nil {
		req.RespondWithStatus(txnStatus_NearNonceError)
		return
	}

	tp.logger.Info().Msgf("sending txn [%s] - sender: [%s], eth nonce: [%d], near nonce: [%d]", req.hash, req.sender.Hex(), req.nonce, nonce)

	go func(key string, req *TxnReq) {
		resp, err := tp.account.FunctionCallWithMultiActionAndKeyAndNonce(utils.AccountId, "submit", key,
			[][]byte{req.rawTxn}, tp.config.GasForNearTxsCall, nonce, *tp.config.DepositForNearTxsCall)
		req.Respond(resp, err)
	}(key, req)
}

// eval evaluates the txn response and decides whether to retry txn req or not
func (tp *TxnProcessor) eval(req *TxnReq, resp *TxnResp) (retry bool) {

	if resp.err == nil {
		isFailure, mapFailure, err := isTxnResponseFailure(resp.resp)
		if err != nil {
			resp.status = txnStatus_UnhandledError
			tp.logger.Info().Msgf("isTxnResponseFailure error for nonce [%d] -> %s", req.nonce, err.Error()) // TODO: make the log DEBUG level
		} else if isFailure {
			resp.status = getTxnStatusForFailureType(mapFailure)
			tp.logger.Info().Msgf("isTxnResponseFailure detected failure [%d] for nonce [%d]", resp.status, req.nonce) // TODO: make the log DEBUG level
		} else {
			if req.retransmit {
				//retransmit succeeded which shows that previous Near Timeout failed. Just for debug purposes
				tp.logger.Info().Msgf("SUCCESS after Near TIMEOUT for nonce [%d]", req.nonce) // TODO: remove/commnet out during production
			}

			tp.logger.Info().Msgf("SUCCESS for nonce [%d]", req.nonce) // TODO: make the log DEBUG level
			return false
		}
	} else {
		if resp.status == txnStatus_NearNonceError {
			req.retransmit = true
			tp.logger.Info().Msgf("Near nonce retrievel error, retransmit is triggered for eth nonce [%d]", req.nonce) // TODO: make the log DEBUG level
		} else if resp.status != txnStatus_Any { // if resp.status is a valid txnStatus_X, no manuplation is needed
			tp.logger.Info().Msgf("txnStatus error [%d] received for eth nonce [%d]", resp.status, req.nonce) // TODO: make the log DEBUG level
		} else if strings.Contains(resp.err.Error(), "InvalidNonce:map") {
			resp.status = txnStatus_NearNonceError
			tp.logger.Info().Msgf("Near nonce low error for eth nonce [%d] -> %s", req.nonce, resp.err.Error()) // TODO: make the log DEBUG level
		} else if strings.Contains(resp.err.Error(), "NonceTooLarge") {
			resp.status = txnStatus_NearNonceTooLarge
			tp.logger.Info().Msgf("Near nonce too large error for eth nonce [%d]", req.nonce) // TODO: make the log DEBUG level
		} else if strings.Contains(resp.err.Error(), "Server error: Timeout") {
			req.retransmit = true
			resp.status = txnStatus_NearTimeOutError
			tp.logger.Info().Msgf("Near timeout error occured for eth nonce [%d]", req.nonce) // TODO: make the log DEBUG level

		} else {
			// response error is either encoding/decoding error thrown from near-api-go library or a near rpc error (like INVALID_TRANSACTION, PARSE_ERROR)
			// since retry flow is not useful for these error cases, we trigger nonce updates solely as a precaution
			resp.status = txnStatus_UnhandledError
			tp.logger.Info().Msgf("Unhandled error case for eth nonce [%d]", req.nonce) // TODO: make the log DEBUG level
		}
	}

	act := statusActions[resp.status]
	if act.updateNearCache || act.updateEthCache || (act.backoffMs != time.Duration(0)) {
		tp.retryMutex[req.processorIndex].Lock()
		tp.circuitBreaker[req.processorIndex] = true
		defer func() {
			tp.retryMutex[req.processorIndex].Unlock()
			tp.circuitBreaker[req.processorIndex] = false
		}()
	}

	retry = act.retry
	if act.updateNearCache {
		tp.updateNearNonceCache(tp.keys[req.processorIndex])
	}
	if act.updateEthCache {
		updated, err := tp.updateEthNonceCache(req.sender)
		// retry should be forced (return true) if eth nonce cache is not updated or error returned
		if err != nil || !updated {
			return true
		}
		// after proper cache update, actual nonce can be higher than request nonce,
		// in this case there is no reason to retry the request just send response with actual error evaluated above
		if tp.ethNonceCache[req.sender.Hex()].nonce > req.nonce {

			// If receieved error is ERR_INCORRECT_NONCE after a re-transmit
			// txnStatus_EthNonceError response shows that first operation succeeded
			if req.retransmit && resp.status == txnStatus_EthNonceError {
				req.RespondWithStatus(txnStatus_Any)
				resp.resp = success_case_for_retransmit
				tp.logger.Info().Msgf("ETH_NONCE_ERROR after re-transmit for nonce [%d]", req.nonce) // TODO: make the log DEBUG level
			}

			return false
		}
	}

	if resp.ts.Add(time.Millisecond * act.backoffMs).Before(time.Now()) {
		time.Sleep(time.Millisecond * act.backoffMs)
	}

	return retry
}

// nextNearNonce returns the next near nonce from cache, if there is no matching key in cache then it does a Near RPC to
// fetch it.
//
// Note that, this function is not thread-safe and successive calls increment the cache value so this
// function is not suitable for view operations.
func (tp *TxnProcessor) nextNearNonce(key string) (uint64, error) {
	var ok bool
	var nc *nonceCache
	if nc, ok = tp.nearNonceCache[key]; !ok {
		nonce, err := tp.account.ViewNonce(key)
		if err != nil {
			return 0, err
		}
		nc = &nonceCache{
			nonce:      nonce + 1,
			updateTime: time.Now().Add(time.Second * (-minNonceUpdateIntervalSeconds)),
		}
		tp.nearNonceCache[key] = nc
	} else {
		nc.nonce += 1
	}

	return nc.nonce, nil
}

func (tp *TxnProcessor) updateNearNonceCache(key string) error {

	if nc, ok := tp.nearNonceCache[key]; ok {
		if time.Now().Before(nc.updateTime.Add(minNonceUpdateIntervalSeconds * time.Second)) {
			return nil
		}
	} else {
		tp.nearNonceCache[key] = &nonceCache{updateTime: time.Now().Add(time.Second * time.Duration(-minNonceUpdateIntervalSeconds))}
	}

	nonce, err := tp.account.ViewNonce(key)
	if err == nil {
		// nonce is incremented based on the queue capacity
		tp.nearNonceCache[key].nonce = nonce + qCap
		tp.nearNonceCache[key].updateTime = time.Now()
	} else {
		tp.logger.Info().Msgf("near nonce update is unseccessfull - key[%s], new nonce: [%d]", key, nonce)
	}
	tp.logger.Info().Msgf("near nonce update - key[%s], new nonce: [%d]", key, tp.nearNonceCache[key].nonce)
	return err
}

// updateEthNonceCache updates ethereum nonce cache if last update time satisfies the minNonceUpdateIntervalSeconds interval
//	returns false, error if no update performed
// 	returns true, error if updated
func (tp *TxnProcessor) updateEthNonceCache(address common.Address) (bool, error) {
	key := address.Hex()
	if nc, ok := tp.ethNonceCache[key]; ok {
		if time.Now().Before(nc.updateTime.Add(minNonceUpdateIntervalSeconds * time.Second)) {
			return false, nil
		}
	} else {
		tp.ethNonceCache[key] = &nonceCache{updateTime: time.Now().Add(time.Second * time.Duration(-minNonceUpdateIntervalSeconds))}
	}

	lbn := common.LatestBlockNumber
	bn := &lbn

	resp, err := tp.account.ViewFunction(utils.AccountId, "get_nonce", address.Bytes(), bn.Int64())
	if err != nil {
		return false, err
	}

	engineResp, err := engine.NewQueryResult(resp)
	if err != nil {
		return false, err
	}

	n, err := engineResp.ToUint256Response()
	if err != nil {
		return false, err
	}

	tp.ethNonceCache[key].nonce = n.Uint64()
	tp.ethNonceCache[key].updateTime = time.Now()
	tp.logger.Info().Msgf("eth nonce update - address[%s], new nonce: [%d]", key, n.Uint64())
	return true, nil
}

// isTxnResponseFailure process the response to return
//	true, failure_map, nil -> for failure case
//	false, nil, nil -> for success case
//	false, nil, err -> for invalid response
func isTxnResponseFailure(resp interface{}) (bool, interface{}, error) {
	if status, ok := resp.(map[string]interface{})["status"]; ok {
		if val, ok2 := status.(map[string]interface{})["Failure"]; ok2 {
			return ok2, val, nil
		} else if _, ok3 := status.(map[string]interface{})["SuccessValue"]; ok3 {
			return false, nil, nil
		} else {
			return false, nil, fmt.Errorf("ERR_INVALID_TXN_RESPONSE")
		}

	}
	return false, nil, fmt.Errorf("ERR_INVALID_TXN_RESPONSE")
}

// getTxnStatusForFailureType process the failure map according to the statusMappingArray
//	returns txnStatus_Any if no match
//	returns the provided status for full match
func getTxnStatusForFailureType(mapFailure interface{}) txnStatus {
	returnStatus := txnStatus_Any
	p := mapFailure
	for _, obj := range statusMappingArray {
		for i, v := range obj.fields {
			if (i + 1) == len(obj.fields) {
				if errMsg, ok2 := p.(string); ok2 {
					if v == "*" {
						returnStatus = obj.status
					} else if strings.Contains(errMsg, v) {
						returnStatus = obj.status
					} else {
						returnStatus = txnStatus_Any
					}
				}
			} else if child, ok := p.(map[string]interface{})[v]; ok {
				p = child
			} else {
				returnStatus = txnStatus_Any
				break
			}
		}
		if returnStatus != txnStatus_Any {
			break
		}
		p = mapFailure
	}
	return returnStatus
}
