package endpoint

import (
	"context"
	"encoding/json"
	"net"
	events "relayer2-base/rpcnode/github-ethereum-go-ethereum/events"
	"relayer2-base/types/request"
	"relayer2-base/types/response"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
)

// Please note that this file requires the initializations done in the "Main" function of "engineEth_test.go" file

func TestUnsupportedNotifications(t *testing.T) {
	// Create new EventsForGoEth
	eventsEth := NewEventsForGoEth(baseEndpoint, nil)

	ctx := context.Background()
	data := []struct {
		name           string
		api            string
		method         string
		args           []interface{}
		expectedResult string
	}{
		{"test eth_newHeads unsupported notification", "eth_newHeads", "NewHeads", []interface{}{ctx}, "notifications not supported"},
		{"test eth_logs unsupported notification", "eth_logs", "Logs", []interface{}{ctx, request.LogSubscriptionOptions{}}, "notifications not supported"},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			// Call the target api
			resp, err := Invoke(ctx, eventsEth, d.method, d.args...)

			// Compare the retrieved response/error and expected result
			switch v := resp.(type) {
			case *string:
				if d.expectedResult == "anyHex" {
					tmp := (*v)[0:2]
					if !(tmp == "0x" || tmp[0:1] == "0X") {
						t.Errorf("incorrect response: expected %s, got %s", d.expectedResult, *v)
					}
				} else if *v != d.expectedResult {
					t.Errorf("incorrect response: expected %s, got %s", d.expectedResult, *v)
				}
			default:
				if err.Error() != d.expectedResult {
					t.Errorf("incorrect response: expected %s, got %s", d.expectedResult, err.Error())
				}
			}
		})
	}
}

func TestNewHeadsFlow(t *testing.T) {
	var (
		server                 = rpc.NewServer()
		clientConn, serverConn = net.Pipe()
		out                    = json.NewEncoder(clientConn)
	)
	eb := events.NewEventBroker()
	go eb.Start()
	// Create new EventsForGoEth
	eventsEth := NewEventsForGoEth(baseEndpoint, eb)
	// setup and start server
	if err := server.RegisterName("eth", eventsEth); err != nil {
		t.Fatalf("unable to register events for go-ethereum service %v", err)
	}
	go server.ServeCodec(rpc.NewCodec(serverConn), 0)
	defer server.Stop()

	// create subscriptions and send events
	requestNewHeads := map[string]interface{}{
		"id":      1,
		"method":  "eth_subscribe",
		"version": "2.0",
		"params":  []interface{}{"newHeads"},
	}
	if err := out.Encode(&requestNewHeads); err != nil {
		t.Fatalf("Could not create newHeads subscription: %v", err)
	}

	timeout := time.After(2 * time.Second)
	trigger := time.After(750 * time.Millisecond)
	for {
		select {
		case <-trigger:
			eventsEth.newHeadsCh <- &response.Block{}
			trigger = time.After(750 * time.Millisecond)
		case <-timeout:
			if len(eventsEth.newHeadsCh) > 0 {
				t.Error("newHeads events not consumed")
			}
			return
		}
	}
}

func TestLogsFlow(t *testing.T) {
	var (
		server                 = rpc.NewServer()
		clientConn, serverConn = net.Pipe()
		out                    = json.NewEncoder(clientConn)
	)
	eb := events.NewEventBroker()
	go eb.Start()
	// Create new EventsForGoEth
	eventsEth := NewEventsForGoEth(baseEndpoint, eb)
	// setup and start server
	if err := server.RegisterName("eth", eventsEth); err != nil {
		t.Fatalf("unable to register events for go-ethereum service %v", err)
	}
	go server.ServeCodec(rpc.NewCodec(serverConn), 0)
	defer server.Stop()

	// create subscriptions and send events
	requestLogs := map[string]interface{}{
		"id":      2,
		"method":  "eth_subscribe",
		"version": "2.0",
		"params":  []interface{}{"logs", request.LogSubscriptionOptions{}},
	}
	if err := out.Encode(&requestLogs); err != nil {
		t.Fatalf("Could not create logs subscription: %v", err)
	}

	timeout := time.After(2 * time.Second)
	trigger := time.After(750 * time.Millisecond)
	for {
		select {
		case <-trigger:
			tmp := response.Log{}
			eventsEth.logsCh <- []*response.Log{&tmp}
			trigger = time.After(750 * time.Millisecond)
		case <-timeout:
			if len(eventsEth.logsCh) > 0 {
				t.Error("logs events not consumed")
			}
			return
		}
	}
}
