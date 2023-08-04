package endpoint

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aurora-is-near/near-api-go"
	"github.com/aurora-is-near/relayer2-base/endpoint"
	"github.com/aurora-is-near/relayer2-base/types/common"
	"github.com/aurora-is-near/relayer2-base/types/engine"
	errs "github.com/aurora-is-near/relayer2-base/types/errors"
	"github.com/aurora-is-near/relayer2-base/utils"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

var (
	beforeGenesisError   = "DB Not Found Error"
	beforeAuroraError    = "does not exist while viewing"
	tooManyRequestsError = "429"
)

type SenderKeyMapper interface {
	Map(sender string, totalNumKeys int) (keyIndex int)
}

// CRC32Mapper is a signer to key mapper implementation that uses crc32 hash to consistently distributes signers to keys.
// This ensures that same signers are mapped to same key
type CRC32Mapper struct{}

func (m CRC32Mapper) Map(sender string, totalNumKeys int) int {
	idx := crc32.ChecksumIEEE([]byte(sender)) % uint32(totalNumKeys)
	return int(idx)
}

// RoundRobinMapper is a signer to key mapper implementation that equally distributes signers to keys
type RoundRobinMapper struct {
	offset uint32
}

func (m RoundRobinMapper) Map(sender string, totalNumKeys int) int {
	return m.rrMap(totalNumKeys)
}

func (m RoundRobinMapper) rrMap(totalNumKeys int) int {
	offset := atomic.AddUint32(&m.offset, 1) - 1
	idx := offset % uint32(totalNumKeys)
	return int(idx)
}

type EngineEth struct {
	*endpoint.Endpoint
	signer *near.Account
	keys   []string
	mapper SenderKeyMapper
}

func NewEngineEth(ep *endpoint.Endpoint) *EngineEth {
	if ep.Config.EngineConfig.NearNetworkID == "" || ep.Config.EngineConfig.NearNodeURL == "" || ep.Config.EngineConfig.Signer == "" {
		ep.Logger.Panic().Msg("Near settings in the config file under `endpoint->engine` should be checked")
	}

	// Establish engine communication and auth the near account
	nearCfg := &near.Config{
		NetworkID:                ep.Config.EngineConfig.NearNetworkID,
		NodeURL:                  ep.Config.EngineConfig.NearNodeURL,
		FunctionKeyPrefixPattern: ep.Config.EngineConfig.FunctionKeyPrefixPattern,
	}
	if ep.Config.EngineConfig.SignerKey != "" {
		nearCfg.KeyPath = ep.Config.EngineConfig.SignerKey
	}
	nearConn := near.NewConnection(nearCfg.NodeURL)
	nearAccount, err := near.LoadAccount(nearConn, nearCfg, ep.Config.EngineConfig.Signer)
	if err != nil {
		ep.Logger.Panic().Err(err).Msg("failed to load Near account")
	}

	var mapper SenderKeyMapper
	switch ep.Config.EngineConfig.FunctionKeyMapper {
	case "CRC32":
		mapper = CRC32Mapper{}
	case "RoundRobin":
		mapper = RoundRobinMapper{}
	}

	eEth := &EngineEth{
		Endpoint: ep,
		signer:   nearAccount,
		keys:     nearAccount.GetVerifiedAccessKeys(),
		mapper:   mapper,
	}

	return eEth
}

// ChainId returns the chain id of the current network
//
//	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
//	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
//	On any param returns error code '-32602' with custom message.
func (e *EngineEth) ChainId(ctx context.Context) (*common.Uint256, error) {
	return endpoint.Process(ctx, "eth_chainId", e.Endpoint, func(ctx context.Context) (*common.Uint256, error) {
		return e.chainId(ctx)
	})
}

func (e *EngineEth) chainId(_ context.Context) (*common.Uint256, error) {
	resp, err := e.signer.ViewFunction(utils.AccountId, "get_chain_id", []byte{}, nil)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getUint256ResultFromEngineResponse(resp)
}

// GetCode returns the compiled smart contract code, if any, at a given address
//
//	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
//	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
//	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) GetCode(ctx context.Context, address common.Address, bNumOrHash *common.BlockNumberOrHash) (*string, error) {
	return endpoint.Process(ctx, "eth_getCode", e.Endpoint, func(ctx context.Context) (*string, error) {
		return e.getCode(ctx, address, bNumOrHash)
	}, address, bNumOrHash)
}

func (e *EngineEth) getCode(ctx context.Context, address common.Address, bNumOrHash *common.BlockNumberOrHash) (*string, error) {
	bn, err := e.blockNumberByNumberOrHash(ctx, bNumOrHash)
	if err != nil {
		return nil, err
	}

	resp, err := e.signer.ViewFunction(utils.AccountId, "get_code", address.Bytes(), bn.Int64())
	if err != nil {
		// Return "0x" for the blocks before Aurora account or before Genesis
		if strings.Contains(err.Error(), beforeAuroraError) || strings.Contains(err.Error(), beforeGenesisError) {
			return utils.Constants.Response0x(), nil
		}
		return nil, &errs.GenericError{Err: err}
	}
	return getStringResultFromEngineResponse(resp)
}

// GetBalance returns the balance of the account of given address
//
//	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
//	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
//	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) GetBalance(ctx context.Context, address common.Address, bNumOrHash *common.BlockNumberOrHash) (*common.Uint256, error) {
	return endpoint.Process(ctx, "eth_getBalance", e.Endpoint, func(ctx context.Context) (*common.Uint256, error) {
		return e.getBalance(ctx, address, bNumOrHash)
	}, address, bNumOrHash)
}

func (e *EngineEth) getBalance(ctx context.Context, address common.Address, bNumOrHash *common.BlockNumberOrHash) (*common.Uint256, error) {
	bn, err := e.blockNumberByNumberOrHash(ctx, bNumOrHash)
	if err != nil {
		return nil, err
	}

	resp, err := e.signer.ViewFunction(utils.AccountId, "get_balance", address.Bytes(), bn.Int64())
	if err != nil {
		// Return "0x0" for the blocks before Aurora account or before Genesis
		if strings.Contains(err.Error(), beforeAuroraError) || strings.Contains(err.Error(), beforeGenesisError) {
			return utils.Constants.ZeroUint256(), nil
		}
		return nil, &errs.GenericError{Err: err}
	}
	return getUint256ResultFromEngineResponse(resp)
}

// GetTransactionCount returns the number of transactions sent from an address
//
//	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
//	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
//	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) GetTransactionCount(ctx context.Context, address common.Address, bNumOrHash *common.BlockNumberOrHash) (*common.Uint256, error) {
	return endpoint.Process(ctx, "eth_getTransactionCount", e.Endpoint, func(ctx context.Context) (*common.Uint256, error) {
		return e.getTransactionCount(ctx, address, bNumOrHash)
	}, address, bNumOrHash)
}

func (e *EngineEth) getTransactionCount(ctx context.Context, address common.Address, bNumOrHash *common.BlockNumberOrHash) (*common.Uint256, error) {
	bn, err := e.blockNumberByNumberOrHash(ctx, bNumOrHash)
	if err != nil {
		return nil, err
	}

	resp, err := e.signer.ViewFunction(utils.AccountId, "get_nonce", address.Bytes(), bn.Int64())
	if err != nil {
		// Return "0x0" for the blocks before Aurora account or before Genesis
		if strings.Contains(err.Error(), beforeAuroraError) || strings.Contains(err.Error(), beforeGenesisError) {
			return utils.Constants.ZeroUint256(), nil
		} else if strings.Contains(err.Error(), tooManyRequestsError) {
			return nil, errors.New("engine error 429, too many requests received")
		}
		return nil, &errs.GenericError{Err: err}
	}
	return getUint256ResultFromEngineResponse(resp)
}

// GetStorageAt returns the value from a storage position at a given address
//
//	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
//	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
//	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) GetStorageAt(ctx context.Context, address common.Address, storageSlot common.Uint256, bNumOrHash *common.BlockNumberOrHash) (*string, error) {
	return endpoint.Process(ctx, "eth_getStorageAt", e.Endpoint, func(ctx context.Context) (*string, error) {
		return e.getStorageAt(ctx, address, storageSlot, bNumOrHash)
	}, address, bNumOrHash)
}

func (e *EngineEth) getStorageAt(ctx context.Context, address common.Address, storageSlot common.Uint256, bNumOrHash *common.BlockNumberOrHash) (*string, error) {
	argsBuf, err := formatGetStorageAtArgsForEngine(address, storageSlot)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	bn, err := e.blockNumberByNumberOrHash(ctx, bNumOrHash)
	if err != nil {
		return nil, err
	}

	resp, err := e.signer.ViewFunction(utils.AccountId, "get_storage_at", argsBuf, bn.Int64())
	if err != nil {
		// Return "0x" for the blocks before Aurora account or before Genesis
		if strings.Contains(err.Error(), beforeAuroraError) || strings.Contains(err.Error(), beforeGenesisError) {
			return utils.Constants.ZeroStrUint256(), nil
		}
		return nil, &errs.GenericError{Err: err}
	}
	return getStringResultFromEngineResponse(resp)
}

// Call executes a new message call immediately without creating a transaction on the blockchain
//
//	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
//	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
//	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) Call(ctx context.Context, txs engine.TransactionForCall, bNumOrHash *common.BlockNumberOrHash) (*string, error) {
	return endpoint.Process(ctx, "eth_call", e.Endpoint, func(ctx context.Context) (*string, error) {
		return e.call(ctx, txs, bNumOrHash)
	}, txs, bNumOrHash)
}

func (e *EngineEth) call(ctx context.Context, txs engine.TransactionForCall, bNumOrHash *common.BlockNumberOrHash) (*string, error) {
	err := txs.Validate()
	if err != nil {
		return nil, err
	}
	argsBuf, err := formatCallArgsForEngine(txs)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	bn, err := e.blockNumberByNumberOrHash(ctx, bNumOrHash)
	if err != nil {
		return nil, err
	}

	resp, err := e.signer.ViewFunction(utils.AccountId, "view", argsBuf, bn.Int64())
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getCallResultFromEngineResponse(resp)
}

// SendRawTransaction submits a raw transaction to engine either asynchronously or synchronously based on the configuration
//
//	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
//	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
//	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) SendRawTransaction(ctx context.Context, txs common.DataVec) (*string, error) {
	return endpoint.Process(ctx, "eth_sendRawTransaction", e.Endpoint, func(ctx context.Context) (*string, error) {
		return e.sendRawTransaction(ctx, txs)
	}, txs)
}

func (e *EngineEth) sendRawTransaction(_ context.Context, txs common.DataVec) (*string, error) {

	// check transaction data and return error if any issues (like low gas price or gas limit)
	sender, err := validateRawTransaction(txs.Bytes(), e.Config.EngineConfig)
	if err != nil {
		return nil, &errs.InvalidParamsError{Message: err.Error()}
	}

	key := e.keys[e.mapper.Map(sender.Hex(), len(e.keys))]
	e.Endpoint.Logger.Debug().Msgf("sender: [%s], key: [%s]", sender.Hex(), key)

	// Call either async or sync version of sendRawTransaction according to the configuration parameter
	if e.Config.EngineConfig.AsyncSendRawTxs {
		return e.asyncSendRawTransactionWithPublicKey(txs.Bytes(), key)
	} else {
		return e.syncSendRawTransactionWithPublicKey(txs.Bytes(), key)
	}
}

// asyncSendRawTransaction submits a raw transaction to engine asynchronously
//
// Deprecated: Use asyncSendRawTransactionWithPublicKey instead
func (e *EngineEth) asyncSendRawTransaction(txsBytes []byte) (*string, error) {

	resp, err := e.sendRawTransactionWithRetry(txsBytes)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	txsHash := utils.CalculateKeccak256(txsBytes)
	e.Logger.Info().Msgf("Near txs hash is: %s, for Eth txs hash: %s", *resp, txsHash)

	return &txsHash, nil
}

// asyncSendRawTransactionWithPublicKey submits a raw transaction to engine asynchronously. Transaction is signed with the key pair defined
// by the publicKey arg
func (e *EngineEth) asyncSendRawTransactionWithPublicKey(txsBytes []byte, publicKey string) (*string, error) {

	resp, err := e.sendRawTransactionWithRetryWithPublicKey(txsBytes, publicKey)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	txsHash := utils.CalculateKeccak256(txsBytes)
	e.Logger.Info().Msgf("Near txs hash is: %s, for Eth txs hash: %s", *resp, txsHash)

	return &txsHash, nil
}

// syncSendRawTransaction submits a raw transaction to engine synchronously
//
// Deprecated: Use syncSendRawTransactionWithPublicKey instead
func (e *EngineEth) syncSendRawTransaction(txsBytes []byte) (*string, error) {

	txsHash := utils.CalculateKeccak256(txsBytes)
	amount := e.Config.EngineConfig.DepositForNearTxsCall
	resp, err := e.signer.FunctionCall(utils.AccountId, "submit", txsBytes, e.Config.EngineConfig.GasForNearTxsCall, *amount)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getTxsResultFromEngineResponse(resp, txsHash)
}

// syncSendRawTransactionWithPublicKey submits a raw transaction to engine synchronously. Transaction is signed with the key pair defined
// by the publicKey arg
func (e *EngineEth) syncSendRawTransactionWithPublicKey(txsBytes []byte, publicKey string) (*string, error) {

	txsHash := utils.CalculateKeccak256(txsBytes)
	amount := e.Config.EngineConfig.DepositForNearTxsCall
	resp, err := e.signer.FunctionCallWithMultiActionAndKey(utils.AccountId, "submit", publicKey, [][]byte{txsBytes}, e.Config.EngineConfig.GasForNearTxsCall, *amount)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getTxsResultFromEngineResponse(resp, txsHash)
}

// sendRawTransactionWithRetry send the Txs with a constant configurable retry count and duration in case of error
func (e *EngineEth) sendRawTransactionWithRetry(txsBytes []byte) (*string, error) {
	amount := e.Config.EngineConfig.DepositForNearTxsCall
	gas := e.Config.EngineConfig.GasForNearTxsCall
	waitTimeMs := e.Config.EngineConfig.RetryWaitTimeMsForNearTxsCall
	retryNumber := e.Config.EngineConfig.RetryNumberForNearTxsCall

	for i := 0; i < retryNumber; i++ {
		resp, err := e.signer.FunctionCallAsync(utils.AccountId, "submit", txsBytes, gas, *amount)
		if err == nil {
			return &resp, nil
		}
		if i < retryNumber-1 {
			e.Logger.Error().Msgf("sendRawTxs error on iteration %d: %s", i, err.Error())
			time.Sleep(time.Duration(waitTimeMs) * time.Millisecond)
		}
	}
	return nil, errors.New("sendRawTransaction: maximum retries reached")
}

// sendRawTransactionWithRetryWithPublicKey send the Txs with a constant configurable retry count and duration in case of error
func (e *EngineEth) sendRawTransactionWithRetryWithPublicKey(txsBytes []byte, publicKey string) (*string, error) {
	amount := e.Config.EngineConfig.DepositForNearTxsCall
	gas := e.Config.EngineConfig.GasForNearTxsCall
	waitTimeMs := e.Config.EngineConfig.RetryWaitTimeMsForNearTxsCall
	retryNumber := e.Config.EngineConfig.RetryNumberForNearTxsCall

	for i := 0; i < retryNumber; i++ {
		resp, err := e.signer.FunctionCallAsyncWithMultiActionAndKey(utils.AccountId, "submit", publicKey, [][]byte{txsBytes}, gas, *amount)
		if err == nil {
			return &resp, nil
		}
		if i < retryNumber-1 {
			e.Logger.Error().Msgf("sendRawTxs error on iteration %d: %s", i, err.Error())
			time.Sleep(time.Duration(waitTimeMs) * time.Millisecond)
		}
	}
	return nil, errors.New("sendRawTransaction: maximum retries reached")
}

func (e *EngineEth) blockNumberByNumberOrHash(ctx context.Context, bNumOrHash *common.BlockNumberOrHash) (*common.BN64, error) {
	if bNumOrHash == nil {
		bn := common.LatestBlockNumber
		return &bn, nil
	}
	if tbn, ok := bNumOrHash.Number(); ok {
		return &tbn, nil
	}
	if hash, ok := bNumOrHash.Hash(); ok {
		tbn, err := e.DbHandler.BlockHashToNumber(ctx, hash)
		if err != nil {
			return nil, err
		}
		bn := common.BN64(*tbn)
		return &bn, nil
	}

	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

// getUint256ResultFromEngineResponse gets the return value from engine and converts it to Uint256 format
func getUint256ResultFromEngineResponse(respArg interface{}) (*common.Uint256, error) {
	engineResult, err := engine.NewQueryResult(respArg)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return engineResult.ToUint256Response()
}

// getStringResultFromEngineResponse gets the return value from engine and converts it to string format
func getStringResultFromEngineResponse(respArg interface{}) (*string, error) {
	engineResult, err := engine.NewQueryResult(respArg)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return engineResult.ToStringResponse()
}

// getCallResultFromEngineResponse gets the return value from engine and converts it to string format
func getCallResultFromEngineResponse(respArg interface{}) (*string, error) {
	status, err := engine.NewTransactionStatus(respArg)
	if err != nil {
		_, ok := err.(*errs.TxsStatusError)
		if ok {
			return nil, err
		} else {
			return nil, &errs.GenericError{Err: err}
		}
	}
	return status.ToResponse()
}

// formatGetStorageAtArgsForEngine gets input address and storage slot arguments
// and returns the serialized buffer to send to the engine
func formatGetStorageAtArgsForEngine(addr common.Address, sSlot common.Uint256) ([]byte, error) {
	buff, err := engine.NewArgsForGetStorageAt(addr, sSlot).Serialize()
	if err != nil {
		return nil, err
	}
	return buff, nil
}

// formatCallArgsForEngine gets the input transaction struct for eth_call, validate its fields
// and returns the serialized buffer to send to the engine
func formatCallArgsForEngine(txs engine.TransactionForCall) ([]byte, error) {
	buff, err := txs.Serialize()
	if err != nil {
		return nil, err
	}
	return buff, nil
}

// getTxsResultFromEngineResponse gets the sendRawTransactionSync response, parse and process the near data structures to be able to generate the rpc response
func getTxsResultFromEngineResponse(respArg interface{}, txsHash string) (*string, error) {
	status, err := engine.NewSubmitStatus(respArg, txsHash)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return status.ToResponse()
}

// validateRawTransaction validates the raw transaction by checking GasPrice and Gas Limit
func validateRawTransaction(rawRxsBytes []byte, cfg endpoint.EngineConfig) (*common.Address, error) {
	txsObj, err := parseTransactionFromBinary(rawRxsBytes)
	if err != nil {
		return nil, err
	}

	// check if provided sender address is valid
	sender, err := extractTransactionSender(txsObj)
	if err != nil {
		return nil, fmt.Errorf("can't extract transaction sender: %v", err)
	}

	// check if provided gas price is bigger than min gas price limit
	if txsObj.GasPrice().Cmp(cfg.MinGasPrice) < 0 {
		return nil, errors.New("gas price too low")
	}

	// check if provided gas limit is bigger than min gas limit
	if txsObj.Gas() < cfg.MinGasLimit {
		return nil, errors.New("intrinsic gas too low")
	}
	return sender, nil
}

// parseTransactionFromBinary decodes the sendRawTransaction data to a go-ethereum transaction structure
func parseTransactionFromBinary(txBinary []byte) (*gethtypes.Transaction, error) {
	var tx gethtypes.Transaction
	if err := tx.UnmarshalBinary(txBinary); err != nil {
		return nil, fmt.Errorf("can't parse transaction: %v", err)
	}

	return &tx, nil
}

// extractTransactionSender decodes the sendRawTransaction sender data and returns if the operation false
func extractTransactionSender(tx *gethtypes.Transaction) (*common.Address, error) {
	addr, err := gethtypes.Sender(gethtypes.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return nil, err
	}
	sender := common.BytesToAddress(addr.Bytes())
	return &sender, nil
}
