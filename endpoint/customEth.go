package endpoint

import (
	"aurora-relayer-go-common/endpoint"
	"aurora-relayer-go-common/utils"
	"context"
)

type CustomEth struct {
	*endpoint.Endpoint
}

func NewCustomEth(endpoint *endpoint.Endpoint) *CustomEth {
	return &CustomEth{endpoint}
}

func (e CustomEth) SendRawTransaction(ctx context.Context, data utils.TxnData) (utils.H256, error) {
	return endpoint.Process(ctx, "eth_sendRawTransaction", e.Endpoint, func(ctx context.Context) (utils.H256, error) {
		return e.sendRawTransaction(ctx, data)
	}, data)
}

func (e CustomEth) sendRawTransaction(_ context.Context, _ utils.TxnData) (utils.H256, error) {
	var txnHash utils.H256
	return txnHash, nil
}
