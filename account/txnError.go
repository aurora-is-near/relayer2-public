package account

import (
	"fmt"
	"time"
)

type txnStatus int

const (
	txnStatus_Any txnStatus = iota
	txnStatus_Reorder
	txnStatus_TimeOutError1
	txnStatus_TimeOutError2
	txnStatus_TimeOutError3
	txnStatus_NearNonceError
	txnStatus_EthNonceError
	txnStatus_EthNonceTooLow
	txnStatus_NearOutOfFundError
	txnStatus_EthOutOfFundError
	txnStatus_Exhausted
)

var statusActions = map[txnStatus]action{
	txnStatus_Any:                {backoffMs: 1000, retry: true, updateEthCache: true, updateNearCache: true},
	txnStatus_Reorder:            {backoffMs: 0, retry: true, updateEthCache: false, updateNearCache: false, error: fmt.Errorf("txn should be ordered again")}, // TODO: error format
	txnStatus_Exhausted:          {backoffMs: 0, retry: false, updateEthCache: false, updateNearCache: false, error: fmt.Errorf("failed to process txn")},      // TODO: error format
	txnStatus_TimeOutError1:      {backoffMs: 1000, retry: true, updateEthCache: true, updateNearCache: true, error: fmt.Errorf("txn time out")},               // TODO: error format
	txnStatus_TimeOutError2:      {backoffMs: 1000, retry: true, updateEthCache: true, updateNearCache: true, error: fmt.Errorf("txn time out")},               // TODO: error format
	txnStatus_TimeOutError3:      {backoffMs: 0, retry: false, updateEthCache: false, updateNearCache: false, error: fmt.Errorf("txn time out")},               // TODO: error format
	txnStatus_NearNonceError:     {backoffMs: 1000, retry: true, updateEthCache: true, updateNearCache: true, error: fmt.Errorf("failed to get Near nonce")},   // TODO: error format
	txnStatus_EthNonceError:      {backoffMs: 1000, retry: true, updateEthCache: true, updateNearCache: true},
	txnStatus_EthNonceTooLow:     {backoffMs: 0, retry: false, updateEthCache: false, updateNearCache: false, error: fmt.Errorf("nonce is too low")},
	txnStatus_NearOutOfFundError: {backoffMs: 0, retry: false, updateEthCache: true, updateNearCache: true},
	txnStatus_EthOutOfFundError:  {backoffMs: 0, retry: false, updateEthCache: true, updateNearCache: false},
}

type action struct {
	retry           bool
	updateNearCache bool
	updateEthCache  bool
	backoffMs       time.Duration
	error           error
}
