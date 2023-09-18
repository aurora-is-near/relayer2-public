package account

import (
	"fmt"
	"time"
)

type txnStatus int

const (
	success_case_for_retransmit = "Success_Retransmit"

	txnStatus_Any txnStatus = iota
	txnStatus_Reorder
	txnStatus_NearNonceError
	txnStatus_NearNonceTooLarge
	txnStatus_EthNonceError
	txnStatus_EthNonceTooLow
	txnStatus_Exhausted
	txnStatus_UnhandledError
	txnStatus_NearTimeOutError
	txnStatus_AuroraContractPanicked
	txnStatus_TimeOutError1
	txnStatus_TimeOutError2
	txnStatus_TimeOutError3
)

var statusActions = map[txnStatus]action{
	txnStatus_Any:                    {backoffMs: 0, retry: false, updateEthCache: false, updateNearCache: false},
	txnStatus_UnhandledError:         {backoffMs: 0, retry: false, updateEthCache: true, updateNearCache: false, error: fmt.Errorf("ERR_INVALID_TXN_RESPONSE")},
	txnStatus_Exhausted:              {backoffMs: 0, retry: false, updateEthCache: false, updateNearCache: false, error: fmt.Errorf("ERR_ENQUEUE_TXN_R")},
	txnStatus_AuroraContractPanicked: {backoffMs: 0, retry: false, updateEthCache: true, updateNearCache: false},
	txnStatus_TimeOutError1:          {backoffMs: 0, retry: false, updateEthCache: true, updateNearCache: false, error: fmt.Errorf("ERR_TXN_TIMEOUT_1_R")},
	txnStatus_TimeOutError2:          {backoffMs: 0, retry: false, updateEthCache: true, updateNearCache: false, error: fmt.Errorf("ERR_TXN_TIMEOUT_2_R")},
	txnStatus_TimeOutError3:          {backoffMs: 0, retry: false, updateEthCache: false, updateNearCache: false, error: fmt.Errorf("ERR_TXN_TIMEOUT_3_R")},
	txnStatus_EthNonceError:          {backoffMs: 0, retry: true, updateEthCache: true, updateNearCache: false, error: fmt.Errorf("ERR_INCORRECT_NONCE")},
	txnStatus_EthNonceTooLow:         {backoffMs: 0, retry: true, updateEthCache: true, updateNearCache: false, error: fmt.Errorf("ERR_INCORRECT_NONCE_R")},
	txnStatus_NearNonceError:         {backoffMs: 0, retry: true, updateEthCache: true, updateNearCache: false, error: fmt.Errorf("ERR_INCORRECT_NEAR_NONCE_R")},
	txnStatus_NearNonceTooLarge:      {backoffMs: 0, retry: true, updateEthCache: true, updateNearCache: true, error: fmt.Errorf("ERR_INCORRECT_NEAR_NONCE_LARGE_R")},
	txnStatus_Reorder:                {backoffMs: 0, retry: true, updateEthCache: false, updateNearCache: false, error: fmt.Errorf("ERR_TXN_ORDER_R")},
	txnStatus_NearTimeOutError:       {backoffMs: 0, retry: true, updateEthCache: false, updateNearCache: false},
}

type action struct {
	retry           bool
	updateNearCache bool
	updateEthCache  bool
	backoffMs       time.Duration
	error           error
}

type txnFailureMapping struct {
	fields []string
	status txnStatus
}

// statusMappingArray holds the tnxStatus for Near Failure status
// Please note that the order of statusMappingArray is important
var statusMappingArray = [6]txnFailureMapping{
	{fields: []string{"ActionError", "kind", "FunctionCallError", "ExecutionError", "ERR_INCORRECT_NONCE"}, status: txnStatus_EthNonceError},
	{fields: []string{"ActionError", "kind", "FunctionCallError", "ExecutionError", "*"}, status: txnStatus_AuroraContractPanicked},
}
