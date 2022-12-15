package endpoint

import (
	"aurora-relayer-go-common/endpoint"
	"aurora-relayer-go-common/types/common"
	"aurora-relayer-go-common/types/engine"
	errs "aurora-relayer-go-common/types/errors"
	"aurora-relayer-go-common/utils"
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/aurora-is-near/near-api-go"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

type EngineEth struct {
	*endpoint.Endpoint
	signer *near.Account
}

func NewEngineEth(ep *endpoint.Endpoint) *EngineEth {
	if ep.Config.EngineConfig.NearNetworkID == "" || ep.Config.EngineConfig.NearNodeURL == "" || ep.Config.EngineConfig.Signer == "" {
		panic("Near settings in the config file under `endpoint->engine` should be checked")
	}

	// Establish engine communication and auth the near account
	nearCfg := &near.Config{
		NetworkID: ep.Config.EngineConfig.NearNetworkID,
		NodeURL:   ep.Config.EngineConfig.NearNodeURL,
	}
	if ep.Config.EngineConfig.SignerKey != "" {
		nearCfg.KeyPath = ep.Config.EngineConfig.SignerKey
	}
	nearcon := near.NewConnection(nearCfg.NodeURL)
	nearaccount, err := near.LoadAccount(nearcon, nearCfg, ep.Config.EngineConfig.Signer)
	if err != nil {
		panic(err)
	}

	eEth := &EngineEth{
		Endpoint: ep,
		signer:   nearaccount,
	}
	return eEth
}

// ChainId returns the chain id of the current network
//
// 	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
// 	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
// 	On any param returns error code '-32602' with custom message.
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
// 	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
// 	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
// 	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) GetCode(ctx context.Context, address common.Address, number *common.BN64) (*string, error) {
	return endpoint.Process(ctx, "eth_getCode", e.Endpoint, func(ctx context.Context) (*string, error) {
		return e.getCode(ctx, address, number)
	}, address, number)
}

func (e *EngineEth) getCode(_ context.Context, address common.Address, number *common.BN64) (*string, error) {
	resp, err := e.signer.ViewFunction(utils.AccountId, "get_code", address.Bytes(), common.BN64ToInt64(number))
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getStringResultFromEngineResponse(resp)
}

// GetBalance returns the balance of the account of given address
//
// 	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
// 	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
// 	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) GetBalance(ctx context.Context, address common.Address, number *common.BN64) (*common.Uint256, error) {
	return endpoint.Process(ctx, "eth_getBalance", e.Endpoint, func(ctx context.Context) (*common.Uint256, error) {
		return e.getBalance(ctx, address, number)
	}, address, number)
}

func (e *EngineEth) getBalance(_ context.Context, address common.Address, number *common.BN64) (*common.Uint256, error) {
	resp, err := e.signer.ViewFunction(utils.AccountId, "get_balance", address.Bytes(), common.BN64ToInt64(number))
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getUint256ResultFromEngineResponse(resp)
}

// GetTransactionCount returns the number of transactions sent from an address
//
// 	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
// 	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
// 	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) GetTransactionCount(ctx context.Context, address common.Address, number *common.BN64) (*common.Uint256, error) {
	return endpoint.Process(ctx, "eth_getTransactionCount", e.Endpoint, func(ctx context.Context) (*common.Uint256, error) {
		return e.getTransactionCount(ctx, address, number)
	}, address, number)
}

func (e *EngineEth) getTransactionCount(_ context.Context, address common.Address, number *common.BN64) (*common.Uint256, error) {
	resp, err := e.signer.ViewFunction(utils.AccountId, "get_nonce", address.Bytes(), common.BN64ToInt64(number))
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getUint256ResultFromEngineResponse(resp)
}

// GetStorageAt returns the value from a storage position at a given address
//
// 	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
// 	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
// 	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) GetStorageAt(ctx context.Context, address common.Address, storageSlot common.Uint256, number *common.BN64) (*common.Uint256, error) {
	return endpoint.Process(ctx, "eth_getStorageAt", e.Endpoint, func(ctx context.Context) (*common.Uint256, error) {
		return e.getStorageAt(ctx, address, storageSlot, number)
	}, address, number)
}

func (e *EngineEth) getStorageAt(_ context.Context, address common.Address, storageSlot common.Uint256, number *common.BN64) (*common.Uint256, error) {
	argsBuf, err := formatGetStorageAtArgsForEngine(address, storageSlot)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}

	resp, err := e.signer.ViewFunction(utils.AccountId, "get_storage_at", argsBuf, common.BN64ToInt64(number))
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getUint256ResultFromEngineResponse(resp)
}

// Call executes a new message call immediately without creating a transaction on the blockchain
//
// 	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
// 	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
// 	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) Call(ctx context.Context, txs engine.TransactionForCall, number *common.BN64) (*string, error) {
	return endpoint.Process(ctx, "eth_call", e.Endpoint, func(ctx context.Context) (*string, error) {
		return e.call(ctx, txs, number)
	}, txs, number)
}

func (e *EngineEth) call(_ context.Context, txs engine.TransactionForCall, number *common.BN64) (*string, error) {
	argsBuf, err := formatCallArgsForEngine(txs)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	resp, err := e.signer.ViewFunction(utils.AccountId, "view", argsBuf, common.BN64ToInt64(number))
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getCallResultFromEngineResponse(resp)
}

// SendRawTransaction submits a raw transaction to engine either asynchronously or synchronously based on the configuration
//
// 	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
// 	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
// 	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) SendRawTransaction(ctx context.Context, txs common.DataVec) (*string, error) {
	return endpoint.Process(ctx, "eth_sendRawTransaction", e.Endpoint, func(ctx context.Context) (*string, error) {
		return e.sendRawTransaction(ctx, txs)
	}, txs)
}

func (e *EngineEth) sendRawTransaction(_ context.Context, txs common.DataVec) (*string, error) {
	// Call either async or sync version of sendRawTransaction according to the configuration parameter
	if e.Config.EngineConfig.AsyncSendRawTxs {
		return e.asyncSendRawTransaction(txs)
	} else {
		return e.syncSendRawTransaction(txs)
	}
}

// asyncSendRawTransaction submits a raw transaction to engine asynchronously
func (e *EngineEth) asyncSendRawTransaction(txsBytes []byte) (*string, error) {
	// check transaction data and return error if any issues (like low gas price or gas limit)
	err := validateRawTransaction(txsBytes, e.Config.EngineConfig)
	if err != nil {
		return nil, &errs.InvalidParamsError{Message: err.Error()}
	}
	resp, err := e.sendRawTransactionWithRetry(txsBytes)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	txsHash := "0x" + hex.EncodeToString(crypto.Keccak256(txsBytes))
	e.Logger.Info().Msgf("Near txs hash is: %s, for Eth txs hash: %s", *resp, txsHash)
	return &txsHash, nil
}

// syncSendRawTransaction submits a raw transaction to engine synchronously
func (e *EngineEth) syncSendRawTransaction(txsBytes []byte) (*string, error) {
	// check transaction data and return error if any issues (like low gas price or gas limit)
	err := validateRawTransaction(txsBytes, e.Config.EngineConfig)
	if err != nil {
		return nil, &errs.InvalidParamsError{Message: err.Error()}
	}

	txsHash := crypto.Keccak256(txsBytes)
	amount := e.Config.EngineConfig.DepositForNearTxsCall
	resp, err := e.signer.FunctionCall(utils.AccountId, "submit", txsBytes, e.Config.EngineConfig.GasForNearTxsCall, *amount)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getTxsResultFromEngineResponse(resp, "0x"+hex.EncodeToString(txsHash))
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
		return nil, &errs.GenericError{Err: err}
	}
	return status.ToResponse()
}

// formatGetStorageAtArgsForEngine gets input address and storage slot arguments
// and returns the serialized buffer to send to the engine
func formatGetStorageAtArgsForEngine(addr common.Address, sSlot common.Uint256) ([]byte, error) {
	argsObj := engine.NewArgsForGetStorageAt().SetFields(addr, sSlot)
	buff, err := argsObj.Serialize()
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
func validateRawTransaction(rawRxsBytes []byte, cfg endpoint.EngineConfig) error {
	txsObj, err := parseRawTransaction(rawRxsBytes)
	if err != nil {
		return errors.New("transaction parameter is not correct")
	}
	// check if provided gas price is bigger than min gas price limit
	if txsObj.GasPrice().Cmp(cfg.MinGasPrice) < 0 {
		return errors.New("gas price too low")
	}
	// check if provided gas limit is bigger than min gas limit
	if txsObj.Gas() < cfg.MinGasLimit {
		return errors.New("intrinsic gas too low")
	}
	return nil
}

// parseRawTransaction decodes the sendRawTransaction data to a go-ethereum transaction structure
func parseRawTransaction(rawTxsBytes []byte) (*gethtypes.Transaction, error) {
	var txs gethtypes.Transaction
	err := rlp.DecodeBytes(rawTxsBytes, &txs)
	if err != nil {
		return nil, err
	}
	return &txs, nil
}
