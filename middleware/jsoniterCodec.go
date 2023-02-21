package middleware

import (
	"github.com/aurora-is-near/relayer2-base/utils"
	"github.com/ethereum/go-ethereum/rpc"
	jsoniter "github.com/json-iterator/go"
	"net/http"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type deadlineCloser struct{}

func (dlc deadlineCloser) SetWriteDeadline(time.Time) error { return nil }
func (dlc deadlineCloser) Close() error {
	return nil
}

// JsonCodec returns a http handler function which can be registered as a middleware.
//
// This handler should be registered as the last handler in the middleware chain (so that next handler is of type
// github.com/ethereum/go-ethereum/rpc.Server (in-proc handler)). It overwrites the json codec of in-proc handler
// and serves the request using in-proc handler
func JsonCodec(next http.Handler) http.Handler {
	utils.RegisterJsoniterEncoders()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv := next.(*rpc.Server)
		enc := json.NewEncoder(w)
		dec := json.NewDecoder(r.Body)

		encode := enc.Encode
		decode := dec.Decode
		srv.ServeCodec(rpc.NewFuncCodec(
			&deadlineCloser{},
			encode,
			decode,
		), 0)
	})
}
