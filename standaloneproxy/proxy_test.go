package standaloneproxy

import (
	"errors"
	"testing"

	"github.com/aurora-is-near/relayer2-base/types/common"
	"github.com/aurora-is-near/relayer2-base/types/primitives"
	"github.com/stretchr/testify/require"
)

func TestBuildRequest(t *testing.T) {
	ttable := []struct {
		method string
		params []any
		want   string
	}{
		{
			method: "foobar",
			params: []any{},
			want:   `{"id":1,"jsonrpc":"2.0","method":"foobar","params":[]}`,
		},
		{
			method: "foobar",
			params: []any{
				primitives.Data32FromHex("0x1"),
			},
			want: `{"id":1,"jsonrpc":"2.0","method":"foobar","params":["0x0100000000000000000000000000000000000000000000000000000000000000"]}`,
		},
		{
			method: "foobar",
			params: []any{
				primitives.Data32FromHex("0x1"),
				primitives.Data32FromHex("0x2"),
				primitives.Data32FromHex("0x3"),
			},
			want: `{"id":1,"jsonrpc":"2.0","method":"foobar","params":["0x0100000000000000000000000000000000000000000000000000000000000000","0x0200000000000000000000000000000000000000000000000000000000000000","0x0300000000000000000000000000000000000000000000000000000000000000"]}`,
		},
	}

	for _, tc := range ttable {
		t.Run(tc.want, func(t *testing.T) {
			got, err := buildRequest(tc.method, tc.params...)
			require.NoError(t, err)
			require.EqualValues(t, tc.want, string(got))
		})
	}
}

func TestParseEstimateGasResponseSuccess(t *testing.T) {
	testCases := []struct {
		name string
		resp []byte
		want common.Uint256
	}{
		{
			name: "success",
			resp: []byte(`{"id":1,"jsonrpc":"2.0","result":21000}`),
			want: common.IntToUint256(21000),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEstimateGasResponse(tc.resp)
			require.NoError(t, err)
			require.EqualValues(t, tc.want, *got)
		})
	}
}

func TestParseEstimateGasResponseError(t *testing.T) {
	testCases := []struct {
		name          string
		resp          []byte
		expectedError error
	}{
		{
			name:          "Error with message",
			resp:          []byte(`{"id":1,"jsonrpc":"2.0","error":{"code":-32000,"message":"Internal error"}}`),
			expectedError: errors.New("engine error: Internal error"),
		},
		{
			name:          "Error with message and data",
			resp:          []byte(`{"id":1,"jsonrpc":"2.0","error":{"code":-32000,"message":"Internal error", "data": "StateMissing"}}`),
			expectedError: errors.New("engine error: StateMissing"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEstimateGasResponse(tc.resp)
			require.Nil(t, got)
			require.EqualValues(t, tc.expectedError, err)
		})
	}
}

func TestHandleEstimateGasError(t *testing.T) {
	testCases := []struct {
		name          string
		input         []byte
		expectedError error
	}{
		{
			name:          "Error with message",
			input:         []byte(`{"error":{"code":-32000,"data":"StateMissing","message":"Internal error"}}`),
			expectedError: errors.New("engine error: StateMissing"),
		},
		{
			name:          "Error without data",
			input:         []byte(`{"error":{"code":-32000,"message":"Internal error"}}`),
			expectedError: errors.New("engine error: Internal error"),
		},
		{
			name:          "Broken JSON",
			input:         []byte(`{"error"}}`),
			expectedError: errors.New("engine error: unknown error occurred"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handleEstimateGasError(tc.input)

			if err == nil || err.Error() != tc.expectedError.Error() {
				t.Errorf("Test %s failed. Got: %v, want: %v", tc.name, err, tc.expectedError)
			}
		})
	}
}
