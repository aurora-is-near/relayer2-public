package account

import (
	"context"
	"sync"
)

type TxnQ[T any] struct {
	q          chan *T
	wg         sync.WaitGroup
	cancelCtx  context.Context
	cancelFunc context.CancelFunc
}

// NewTxnQ creates a new transaction queue with the given capacity
func NewTxnQ[T any](capacity int) (*TxnQ[T], error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	return &TxnQ[T]{
		q:          make(chan *T, capacity),
		wg:         sync.WaitGroup{},
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}, nil
}

// TryEnqueue tries to push data to Q, if Q is full it returns false, otherwise true
func (q *TxnQ[T]) TryEnqueue(txnData *T) bool {
	select {
	case q.q <- txnData:
		return true
	default:
		return false
	}
}

// Enqueue pushes data to Q, if Q is full it blocks until a worker pops a data from Q
func (q *TxnQ[T]) Enqueue(data *T) {
	q.q <- data
}

// Close closes the underlying Q but waits for workers to drain the Q
func (q *TxnQ[T]) Close() {
	close(q.q)
	q.wg.Wait()
}

// Cancel closes the underlying Q without waiting for workers to drain the Q
func (q *TxnQ[T]) Cancel() {
	q.cancelFunc()
}

// StartWorker attaches the given function as a consumer of the TxnQ. Whenever a new data present on the Q this function,
// with data as function parameter, runs. Multiple workers which consume the same Q, can be started by calling this function.
func (q *TxnQ[T]) StartWorker(worker func(*T)) {
	go func() {
		q.wg.Add(1)
		defer q.wg.Done()
		for {
			select {
			case <-q.cancelCtx.Done():
				return
			case data := <-q.q:
				if data != nil {
					worker(data)
				}
				if q.cancelCtx.Err() != nil || data == nil {
					return
				}
			}
		}
	}()
}
