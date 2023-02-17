package endpoint

import (
	"context"
	"github.com/aurora-is-near/relayer2-base/broker"
	"github.com/aurora-is-near/relayer2-base/endpoint"
	eventbroker "github.com/aurora-is-near/relayer2-base/rpcnode/github-ethereum-go-ethereum/events"
	"github.com/aurora-is-near/relayer2-base/types/event"
	"github.com/aurora-is-near/relayer2-base/types/request"

	"github.com/ethereum/go-ethereum/rpc"
)

type EventsForGoEth struct {
	*endpoint.Endpoint
	eventBroker broker.Broker
	newHeadsCh  chan event.Block
	logsCh      chan event.Logs
}

func NewEventsForGoEth(ep *endpoint.Endpoint, eb broker.Broker) *EventsForGoEth {
	return &EventsForGoEth{
		Endpoint:    ep,
		eventBroker: eb,
		newHeadsCh:  make(chan event.Block, eventbroker.NewHeadsChSize),
		logsCh:      make(chan event.Logs, eventbroker.LogsChSize),
	}
}

// NewHeads send a notification each time a new block is appended to the chain, including chain reorganizations.
func (e *EventsForGoEth) NewHeads(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
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
			case <-notifier.Closed():
				e.eventBroker.UnsubscribeFromNewHeads(newHeadsSubs)
				return
			}
		}
	}()

	return rpcSub, nil
}

// Logs send a notification each time logs included in new imported block and match the given filter criteria.
func (e *EventsForGoEth) Logs(ctx context.Context, subOpts request.LogSubscriptionOptions) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
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
			case <-notifier.Closed():
				e.eventBroker.UnsubscribeFromLogs(logsSubs)
				return
			}
		}
	}()

	return rpcSub, nil
}
