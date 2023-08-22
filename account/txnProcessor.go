package account

import (
	"fmt"
	"github.com/aurora-is-near/near-api-go"
	"github.com/aurora-is-near/relayer2-base/endpoint"
	"github.com/aurora-is-near/relayer2-base/log"
	"github.com/aurora-is-near/relayer2-base/types/common"
	"github.com/aurora-is-near/relayer2-base/types/engine"
	"github.com/aurora-is-near/relayer2-base/utils"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
	"sync"
	"time"
)

const (
	qCap            = 1000
	timeoutSeconds  = 30
	timeoutSeconds2 = 2 * timeoutSeconds
)

type TxnProcessor struct {
	mapper         TxnMapper
	config         *endpoint.EngineConfig
	logger         *log.Logger
	account        *near.Account
	ethNonceCache  map[string]uint64
	nearNonceCache map[string]uint64
	keys           []string
	retryMutex     []*sync.Mutex
	ordered        []*TxnQ[TxnReq]
	unordered      []*TxnQ[TxnReq]
	circuitBreaker []bool
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
		ethNonceCache:  make(map[string]uint64),
		nearNonceCache: make(map[string]uint64),
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
	}, nil
}

// Submit starts the transaction processing for TxnReq. It returns a pointer to TxnResp which is a handle to
// get actual transaction response. Also see, TxnResp.Get()
func (tp *TxnProcessor) Submit(req *TxnReq) *TxnResp {

	resp := &TxnResp{
		received: false,
		done:     make(chan struct{}, 1),
	}

	// wait for response with timeout
	go func(req *TxnReq, resp *TxnResp) {
		for {
			select {
			case <-time.After(time.Second * timeoutSeconds):
				resp.SetWithStatus(txnStatus_TimeOutError1)
			case r := <-req.respChan:
				resp.Set(r)
			}
			if !tp.eval(req, resp) {
				break
			}
			tp.unordered[req.processorIndex].TryEnqueue(req)
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

	if nonce, ok := tp.ethNonceCache[req.sender.Hex()]; !ok {
		// if cache miss, then we assume the first received nonce is correct
		tp.ethNonceCache[req.sender.Hex()] = req.nonce
		tp.ordered[req.processorIndex].Enqueue(req)
	} else {
		// if cache hit, compare with request's nonce
		if nonce == req.nonce {
			// nonce match, push request to ordered Q
			tp.ethNonceCache[req.sender.Hex()] = nonce + 1
			tp.ordered[req.processorIndex].Enqueue(req)
		} else if nonce > req.nonce {
			// nonce is low, discard the request (nothing to do)
			req.RespondWithStatus(txnStatus_EthNonceTooLow)
		} else {
			// nonce is high, enqueue this request and wait for another with correct nonce
			if !tp.unordered[req.processorIndex].TryEnqueue(req) {
				req.RespondWithStatus(txnStatus_Exhausted)
			}
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

	tp.logger.Info().Msgf("sending txn - sender: [%s], eth nonce: [%d], near nonce: [%d]", req.sender.Hex(), req.nonce, nonce)

	go func(key string, req *TxnReq) {
		resp, err := tp.account.FunctionCallWithMultiActionAndKeyAndNonce(utils.AccountId, "submit", key,
			[][]byte{req.rawTxn}, tp.config.GasForNearTxsCall, nonce, *tp.config.DepositForNearTxsCall)
		req.Respond(resp, err)
	}(key, req)
}

// eval evaluates the txn response and decides whether to retry txn req or not
func (tp *TxnProcessor) eval(req *TxnReq, resp *TxnResp) (retry bool) {

	if resp.err == nil {
		// TODO: evaluate response message if response message is successful then return false, else set resp.status to txnStatus_X

		status, err := engine.NewSubmitStatus(resp.resp, req.hash)
		if err == nil {
			response, err := status.ToResponse()
			if err == nil {
				if *response == "success" { // TODO: change to proper success check
					return false
				} else {
					resp.status = txnStatus_Any // TODO: this is just a place holder, we may need to evaluate to specific error
				}
			} else {
				resp.status = txnStatus_Any // TODO: this is just a place holder, we may need to evaluate to specific error
			}
		} else {
			resp.status = txnStatus_Any // TODO: this is just a place holder, we may need to evaluate to specific error
		}

	} else {
		// TODO: evaluate err message and set resp.status to txnStatus_X
	}

	act := statusActions[resp.status]
	if act.updateNearCache || act.updateEthCache || (act.backoffMs != time.Duration(0)) {
		tp.retryMutex[req.processorIndex].Lock()
		defer tp.retryMutex[req.processorIndex].Unlock()
	}

	retry = act.retry
	if act.updateNearCache {
		tp.updateNearNonceCache(tp.keys[req.processorIndex])
	}
	if act.updateEthCache {
		tp.updateEthNonceCache(req.sender)
	}

	// after cache update actual nonce can be higher than request nonce, in this case there is no reason to retry this
	// request just send response with actual error evaluated above
	if tp.ethNonceCache[req.sender.Hex()] > req.nonce {
		return false
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
func (tp *TxnProcessor) nextNearNonce(key string) (nonce uint64, err error) {

	var ok bool
	if nonce, ok = tp.nearNonceCache[key]; !ok {
		nonce, err = tp.account.ViewNonce(key)
		if err != nil {
			return 0, err
		}
	}
	nonce++
	tp.nearNonceCache[key] = nonce
	return nonce, nil
}

func (tp *TxnProcessor) updateNearNonceCache(key string) error {

	nonce, err := tp.account.ViewNonce(key)
	if err == nil {
		tp.nearNonceCache[key] = nonce
	}
	tp.logger.Info().Msgf("near nonce update - key[%s], new nonce: [%d]", key, nonce)
	return err
}

func (tp *TxnProcessor) updateEthNonceCache(address common.Address) error {

	lbn := common.LatestBlockNumber
	bn := &lbn

	resp, err := tp.account.ViewFunction(utils.AccountId, "get_nonce", address.Bytes(), bn.Int64())
	if err != nil {
		return err
	}

	engineResp, err := engine.NewQueryResult(resp)
	if err != nil {
		return err
	}

	n, err := engineResp.ToUint256Response()
	if err != nil {
		return err
	}

	tp.ethNonceCache[address.Hex()] = n.Uint64()
	tp.logger.Info().Msgf("eth nonce update - address[%s], new nonce: [%d]", address.Hex(), n.Uint64())
	return nil
}
