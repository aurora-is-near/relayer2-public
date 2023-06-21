package endpoint

import (
	"context"
	"errors"

	"github.com/aurora-is-near/relayer2-base/broker"
	"github.com/aurora-is-near/relayer2-base/endpoint"
	"github.com/aurora-is-near/relayer2-base/rpc"
	"github.com/aurora-is-near/relayer2-base/rpc/node/events"
	"github.com/aurora-is-near/relayer2-base/types/event"
	"github.com/aurora-is-near/relayer2-base/types/request"
)

var (
	ErrNotificationsUnsupported = errors.New("notifications not supported")
)

type EventsEth struct {
	*endpoint.Endpoint
	eventBroker broker.Broker
	newHeadsCh  chan event.Block
	logsCh      chan event.Logs
}

func NewEventsEth(ep *endpoint.Endpoint, eb broker.Broker) *EventsEth {
	return &EventsEth{
		Endpoint:    ep,
		eventBroker: eb,
		newHeadsCh:  make(chan event.Block, events.NewHeadsChSize),
		logsCh:      make(chan event.Logs, events.LogsChSize),
	}
}

// NewHeads send a notification each time a new block is appended to the chain, including chain reorganizations.
func (e *EventsEth) NewHeads(ctx context.Context) (*rpc.ID, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, ErrNotificationsUnsupported
	}
	rpcSub := notifier.CreateSubscription()

	go func() {
		newHeadsSubs := e.eventBroker.SubscribeNewHeads(e.newHeadsCh)
		for {
			select {
			case h := <-e.newHeadsCh:
				notifier.Notify(rpcSub.ID, h)
			case <-rpcSub.Err():
				e.eventBroker.UnsubscribeFromNewHeads(newHeadsSubs)
				return
			}
		}
	}()

	return &rpcSub.ID, nil
}

// Logs send a notification each time logs included in new imported block and match the given filter criteria.
func (e *EventsEth) Logs(ctx context.Context, subOpts request.LogSubscriptionOptions) (*rpc.ID, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return nil, ErrNotificationsUnsupported
	}
	rpcSub := notifier.CreateSubscription()

	go func() {
		logsSubs := e.eventBroker.SubscribeLogs(subOpts, e.logsCh)
		for {
			select {
			case logs := <-e.logsCh:
				for _, log := range logs {
					notifier.Notify(rpcSub.ID, &log)
				}
			case <-rpcSub.Err():
				e.eventBroker.UnsubscribeFromLogs(logsSubs)
				return
			}
		}
	}()

	return &rpcSub.ID, nil
}
