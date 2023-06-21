package endpoint

import (
	"context"
	"testing"

	"github.com/aurora-is-near/relayer2-base/types/request"
)

// Please note that this file requires the initializations done in the "Main" function of "engineEth_test.go" file

func TestUnsupportedNotifications(t *testing.T) {
	// Create new EventsEth
	eventsEth := NewEventsEth(baseEndpoint, nil)

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
