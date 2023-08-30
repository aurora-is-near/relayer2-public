package middleware

import (
	"context"

	"github.com/aurora-is-near/relayer2-base/endpoint"
	"github.com/aurora-is-near/relayer2-base/log"
	"github.com/aurora-is-near/relayer2-base/rpc"
	"github.com/valyala/fasthttp"

	errs "github.com/aurora-is-near/relayer2-base/types/errors"
)

// Proxy redirects the incoming rpc requests to the configured remote server if
// the rpc methods are assigned to be proxied in the configuration file
func Proxy(endpoint *endpoint.Endpoint) rpc.Middleware {
	return func(handler rpc.RpcHandler) rpc.RpcHandler {
		return func(ctx *context.Context, rpcCtx *rpc.RpcContext) *rpc.RpcContext {
			name := rpcCtx.GetMethod()
			if endpoint.Config.ProxyEndpoints[name] {
				log.Log().Debug().Msgf("relaying request: [%s] to remote server", name)
				req := fasthttp.AcquireRequest()
				resp := fasthttp.AcquireResponse()
				defer fasthttp.ReleaseRequest(req)
				defer fasthttp.ReleaseResponse(resp)

				req.SetRequestURI(endpoint.Config.ProxyUrl)
				req.Header.SetContentType(rpc.DefaultContentType)
				req.Header.SetMethod("POST")

				req.SetBody(rpcCtx.GetBody())
				err := fasthttp.Do(req, resp)
				if err != nil {
					log.Log().Error().Err(err).Msgf("failed to call remote server for request: [%s]", name)
					return rpcCtx.SetErrorObject(&errs.GenericError{Err: err})
				}
				return rpcCtx.SetResponse(resp.Body())
			}
			return handler(ctx, rpcCtx)
		}
	}
}
