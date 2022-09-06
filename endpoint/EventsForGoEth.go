package endpoint

import (
	"aurora-relayer-go-common/endpoint"
	eventbroker "aurora-relayer-go-common/rpcnode/github-ethereum-go-ethereum/events"
	"aurora-relayer-go-common/utils"
	"context"

	"github.com/ethereum/go-ethereum/rpc"
)

type EventsForGoEth struct {
	*endpoint.Endpoint
	eventBroker *eventbroker.EventBroker
	newHeadsCh  chan *utils.BlockResponse
	logsCh      chan []*utils.LogResponse
}

func NewEventsForGoEth(ep *endpoint.Endpoint, eb *eventbroker.EventBroker) *EventsForGoEth {
	return &EventsForGoEth{
		Endpoint:    ep,
		eventBroker: eb,
		newHeadsCh:  make(chan *utils.BlockResponse, eventbroker.NewHeadsChSize),
		logsCh:      make(chan []*utils.LogResponse, eventbroker.LogsChSize),
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
func (e *EventsForGoEth) Logs(ctx context.Context, subOpts utils.LogSubscriptionOptions) (*rpc.Subscription, error) {
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
