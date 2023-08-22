package endpoint

import (
	"context"
	"errors"
	"github.com/aurora-is-near/relayer2-base/endpoint"
	"github.com/aurora-is-near/relayer2-base/types/common"
	"github.com/aurora-is-near/relayer2-base/types/engine"
	errs "github.com/aurora-is-near/relayer2-base/types/errors"
	"github.com/aurora-is-near/relayer2-public/account"
)

type EngineEth struct {
	*endpoint.Endpoint
	account *account.Account
}

func NewEngineEth(ep *endpoint.Endpoint) *EngineEth {
	accountConfig := ep.Config.EngineConfig
	acc, err := account.NewAccount(&accountConfig)
	if err != nil {
		ep.Logger.Panic().Err(err).Msg("account could not be created")
	}
	engineEth := &EngineEth{
		Endpoint: ep,
		account:  acc,
	}
	return engineEth
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
	return e.account.ChainId()
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
	return e.account.GetCode(address.Bytes(), bn.Int64())
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
	return e.account.GetBalance(address.Bytes(), bn.Int64())
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
	return e.account.GetTransactionCount(address.Bytes(), bn.Int64())
}

// GetStorageAt returns the value from a storage position at a given address
//
//	On failure to access engine or format error on the response, returns error code '-32000' with custom message.
//	If API is disabled, returns error code '-32601' with message 'the method does not exist/is not available'.
//	On missing or invalid param returns error code '-32602' with custom message.
func (e *EngineEth) GetStorageAt(ctx context.Context, address common.Address, storageSlot common.Uint256, bNumOrHash *common.BlockNumberOrHash) (*string, error) {
	return endpoint.Process(ctx, "eth_getStorageAt", e.Endpoint, func(ctx context.Context) (*string, error) {
		return e.GetStorageAt(ctx, address, storageSlot, bNumOrHash)
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
	return e.account.GetStorageAt(argsBuf, bn.Int64())
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
	return e.account.Call(argsBuf, bn.Int64())
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
	return e.account.SendRawTransaction(txs.Bytes())
}

func (e *EngineEth) Close() {
	e.account.Close()
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
