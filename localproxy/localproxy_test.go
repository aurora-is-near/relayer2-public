package localproxy

import (
	"testing"

	"github.com/aurora-is-near/relayer2-base/types/primitives"
	"github.com/stretchr/testify/require"
)

func TestBuildRequest(t *testing.T) {
	ttable := []struct {
		method string
		params any
		want   string
	}{
		{
			method: "eth_estimateGas",
			params: primitives.Data32FromHex("0x1"),
			want:   `{"id":1,"jsonrpc":"2.0","method":"eth_estimateGas","params":["0x0100000000000000000000000000000000000000000000000000000000000000"]}`,
		},
		{
			method: "foobar",
			params: []interface{}{
				primitives.Data32FromHex("0x1"),
			},
			want: `{"id":1,"jsonrpc":"2.0","method":"foobar","params":["0x0100000000000000000000000000000000000000000000000000000000000000"]}`,
		},
		{
			method: "foobar",
			params: []interface{}{
				primitives.Data32FromHex("0x1"),
				primitives.Data32FromHex("0x2"),
				primitives.Data32FromHex("0x3"),
			},
			want: `{"id":1,"jsonrpc":"2.0","method":"foobar","params":["0x0100000000000000000000000000000000000000000000000000000000000000","0x0200000000000000000000000000000000000000000000000000000000000000","0x0300000000000000000000000000000000000000000000000000000000000000"]}`,
		},
	}

	for _, tc := range ttable {
		t.Run(tc.want, func(t *testing.T) {
			got, err := buildRequest(tc.method, tc.params)
			require.NoError(t, err)
			require.EqualValues(t, tc.want, string(got))
		})
	}
}
