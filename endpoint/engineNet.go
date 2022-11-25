package endpoint

import (
	"aurora-relayer-go-common/endpoint"
	"aurora-relayer-go-common/types/common"
	"context"
)

type EngineNet struct {
	*EngineEth
}

func NewEngineNet(eEth *EngineEth) *EngineNet {
	eNet := &EngineNet{eEth}
	return eNet
}

// Version returns the chain id of the current network. Therefore, directly calls the `chainId`` method under `engineEth` endpoint
func (e *EngineNet) Version(ctx context.Context) (*common.Uint256, error) {
	return endpoint.Process(ctx, "net_version", e.Endpoint, func(ctx context.Context) (*common.Uint256, error) {
		return e.chainId(ctx)
	})
}
