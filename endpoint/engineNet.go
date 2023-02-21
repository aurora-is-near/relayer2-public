package endpoint

import (
	"context"
	"github.com/aurora-is-near/relayer2-base/endpoint"
	"github.com/aurora-is-near/relayer2-base/types"
	errs "github.com/aurora-is-near/relayer2-base/types/errors"
)

type EngineNet struct {
	*EngineEth
}

func NewEngineNet(eEth *EngineEth) *EngineNet {
	eNet := &EngineNet{eEth}
	return eNet
}

// Version returns the chain id of the current network. Therefore, directly calls the `chainId`` method under `engineEth` endpoint
func (e *EngineNet) Version(ctx context.Context) (*types.JsonWrapper[*string], error) {
	return endpoint.Process(ctx, "net_version", e.Endpoint, func(ctx context.Context) (*types.JsonWrapper[*string], error) {
		version, err := e.chainId(ctx)
		if err != nil {
			return nil, &errs.GenericError{Err: err}
		}
		base10str := version.Value.Text(10)
		return types.WrapForJson(&base10str), nil
	})
}
