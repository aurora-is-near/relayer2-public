package account

import (
	"errors"
	"fmt"
	"github.com/aurora-is-near/near-api-go"
	"github.com/aurora-is-near/relayer2-base/endpoint"
	"github.com/aurora-is-near/relayer2-base/types/common"
	"github.com/aurora-is-near/relayer2-base/types/engine"
	errs "github.com/aurora-is-near/relayer2-base/types/errors"
	"github.com/aurora-is-near/relayer2-base/utils"
	"strings"
)

var (
	beforeGenesisError   = "DB Not Found Error"
	beforeAuroraError    = "does not exist while viewing"
	tooManyRequestsError = "429"
)

type Account struct {
	config       *endpoint.EngineConfig
	nearAccount  *near.Account
	txnProcessor *TxnProcessor
}

func NewAccount(config *endpoint.EngineConfig) (*Account, error) {

	if config.NearNetworkID == "" || config.NearNodeURL == "" || config.Signer == "" {
		return nil, fmt.Errorf("one or more Near configuration is missing, please check [nearNetworkID], " +
			"[NearNodeURL], [signer] under `endpoint->engine` in config file")
	}

	// Establish engine communication and auth the near account
	nearCfg := &near.Config{
		NetworkID:                config.NearNetworkID,
		NodeURL:                  config.NearNodeURL,
		FunctionKeyPrefixPattern: config.FunctionKeyPrefixPattern,
	}
	if config.SignerKey != "" {
		nearCfg.KeyPath = config.SignerKey
	}
	nearConn := near.NewConnection(nearCfg.NodeURL)
	nearAccount, err := near.LoadAccount(nearConn, nearCfg, config.Signer)
	if err != nil {
		return nil, fmt.Errorf("failed to load Near account from path: [%s]", nearCfg.KeyPath)
	}

	txnProcessor, err := NewTxnProcessor(config, nearAccount)
	if err != nil {
		return nil, err
	}

	return &Account{
		config:       config,
		nearAccount:  nearAccount,
		txnProcessor: txnProcessor,
	}, nil
}

func (a *Account) Call(txn []byte, bn *int64) (*string, error) {
	resp, err := a.nearAccount.ViewFunction(utils.AccountId, "view", txn, bn)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getCallResultFromEngineResponse(resp)
}

func (a *Account) ChainId() (*common.Uint256, error) {
	resp, err := a.nearAccount.ViewFunction(utils.AccountId, "get_chain_id", []byte{}, nil)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return getUint256ResultFromEngineResponse(resp)
}

func (a *Account) GetBalance(addr []byte, bn *int64) (*common.Uint256, error) {
	resp, err := a.nearAccount.ViewFunction(utils.AccountId, "get_balance", addr, bn)
	if err != nil {
		// Return "0x0" for the blocks before Aurora account or before Genesis
		if strings.Contains(err.Error(), beforeAuroraError) || strings.Contains(err.Error(), beforeGenesisError) {
			return utils.Constants.ZeroUint256(), nil
		}
		return nil, &errs.GenericError{Err: err}
	}
	return getUint256ResultFromEngineResponse(resp)
}

func (a *Account) GetCode(addr []byte, bn *int64) (*string, error) {
	resp, err := a.nearAccount.ViewFunction(utils.AccountId, "get_code", addr, bn)
	if err != nil {
		// Return "0x" for the blocks before Aurora account or before Genesis
		if strings.Contains(err.Error(), beforeAuroraError) || strings.Contains(err.Error(), beforeGenesisError) {
			return utils.Constants.Response0x(), nil
		}
		return nil, &errs.GenericError{Err: err}
	}
	return getStringResultFromEngineResponse(resp)
}

func (a *Account) GetStorageAt(args []byte, bn *int64) (*string, error) {
	resp, err := a.nearAccount.ViewFunction(utils.AccountId, "get_storage_at", args, bn)
	if err != nil {
		// Return "0x" for the blocks before Aurora account or before Genesis
		if strings.Contains(err.Error(), beforeAuroraError) || strings.Contains(err.Error(), beforeGenesisError) {
			return utils.Constants.ZeroStrUint256(), nil
		}
		return nil, &errs.GenericError{Err: err}
	}
	return getStringResultFromEngineResponse(resp)
}

func (a *Account) GetTransactionCount(addr []byte, bn *int64) (*common.Uint256, error) {
	resp, err := a.nearAccount.ViewFunction(utils.AccountId, "get_nonce", addr, bn)
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

func (a *Account) SendRawTransaction(txn []byte) (*string, error) {
	txnReq, err := a.txnProcessor.NewTxnReq(txn)
	txnHash := txnReq.Hash()
	if err != nil {
		return nil, &errs.InvalidParamsError{Message: err.Error()}
	}
	if a.config.AsyncSendRawTxs {
		a.txnProcessor.Submit(txnReq)
		return &txnHash, nil
	} else {
		txnResp := a.txnProcessor.Submit(txnReq)
		resp, err := txnResp.Get()
		if err != nil {
			return nil, err
		}
		return getTxsResultFromEngineResponse(resp, txnHash)
	}
}

// Close gracefully stops the underlying TxnProcessor, also see TxnProcessor.Close()
func (a *Account) Close() {
	a.txnProcessor.Close()
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

// getUint256ResultFromEngineResponse gets the return value from engine and converts it to Uint256 format
func getUint256ResultFromEngineResponse(respArg interface{}) (*common.Uint256, error) {
	engineResult, err := engine.NewQueryResult(respArg)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return engineResult.ToUint256Response()
}

// getTxsResultFromEngineResponse gets the sendRawTransactionSync response, parse and process the near data structures to be able to generate the rpc response
func getTxsResultFromEngineResponse(respArg interface{}, txsHash string) (*string, error) {
	status, err := engine.NewSubmitStatus(respArg, txsHash)
	if err != nil {
		return nil, &errs.GenericError{Err: err}
	}
	return status.ToResponse()
}
