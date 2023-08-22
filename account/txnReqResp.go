package account

import (
	"github.com/aurora-is-near/relayer2-base/types/common"
	"github.com/google/uuid"
	"time"
)

type TxnReq struct {
	processorIndex int
	timestamp      int64
	nonce          uint64
	requestId      uuid.UUID
	sender         common.Address
	respChan       chan *txnResp
	rawTxn         []byte
	hash           string
}

type TxnResp struct {
	*txnResp
	done     chan struct{}
	received bool
}

type txnResp struct {
	status txnStatus
	resp   interface{}
	err    error
	ts     time.Time
}

// Expired returns true if transaction request time plus account.timeoutSeconds is greater than the current time,
// otherwise false
func (req *TxnReq) Expired() bool {
	if time.Now().After(time.Unix(req.timestamp, 0).Add(time.Second * timeoutSeconds)) {
		return true
	}
	return false
}

func (req *TxnReq) RespondWithStatus(status txnStatus) {
	req.respChan <- &txnResp{
		status: status,
		resp:   nil,
		err:    statusActions[status].error,
		ts:     time.Now(),
	}
}

func (req *TxnReq) Respond(resp interface{}, err error) {
	req.respChan <- &txnResp{
		status: txnStatus_Any,
		resp:   resp,
		err:    err,
		ts:     time.Now(),
	}
}

func (req *TxnReq) Hash() string {
	return req.hash
}

// Get is a blocking call which waits for transaction response with timeout. It returns the transaction response or error.
// Successive Get calls on same TxnResp returns same result which was received at first call
func (resp *TxnResp) Get() (interface{}, error) {

	if resp.received {
		return resp.resp, resp.err
	}

	// wait for response with timeout
	select {
	case <-time.After(time.Second * timeoutSeconds2):
		resp.SetWithStatus(txnStatus_TimeOutError2)
	case <-resp.done:
	}
	return resp.resp, resp.err
}

func (resp *TxnResp) SetWithStatus(status txnStatus) {
	resp.status = status
	resp.ts = time.Now()
	resp.err = statusActions[status].error
}

func (resp *TxnResp) Set(txnResp *txnResp) {
	resp.txnResp = txnResp
}

func (resp *TxnResp) Notify() {
	resp.received = true
	resp.done <- struct{}{}
}
