package localproxy

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/aurora-is-near/relayer2-base/cmd"
	"github.com/aurora-is-near/relayer2-base/endpoint"
	"github.com/aurora-is-near/relayer2-base/log"
	"github.com/aurora-is-near/relayer2-base/types/common"
	"github.com/aurora-is-near/relayer2-base/types/engine"
	"github.com/aurora-is-near/relayer2-base/types/response"
	"github.com/buger/jsonparser"
	"github.com/spf13/viper"
)

const configPath = "endpoint.localproxy"

type RPCClient interface {
	TraceTransaction(hash common.H256) (*response.CallFrame, error)
	EstimateGas(tx engine.TransactionForCall, number *common.BN64) (*common.Uint256, error)
	Close() error
}

type Config struct {
	Network string        `mapstructure:"network"`
	Address string        `mapstructure:"address"`
	Timeout time.Duration `mapstructure:"timeout"`
}

func GetConfig() *Config {
	config := &Config{
		Network: "unix",
		Timeout: 5 * time.Second,
	}
	sub := viper.Sub(configPath)
	if sub != nil {
		cmd.BindSubViper(sub, configPath)
		if err := sub.Unmarshal(&config); err != nil {
			log.Log().Warn().Err(err).Msgf("failed to parse configuration [%s] from [%s], "+
				"falling back to defaults", configPath, viper.ConfigFileUsed())
		}
	}
	return config
}

type LocalProxy struct {
	Config *Config
	client RPCClient
}

func New() (*LocalProxy, error) {
	conf := GetConfig()
	client, err := newRPCClient(conf.Address, conf.Timeout)
	if err != nil {
		return nil, err
	}
	return &LocalProxy{conf, client}, err
}

func (l *LocalProxy) Close() error {
	return l.client.Close()
}

// Pre implements endpoint.Processor.
func (l *LocalProxy) Pre(ctx context.Context, name string, _ *endpoint.Endpoint, response *any, args ...any) (context.Context, bool, error) {
	switch name {
	case "debug_traceTransaction":
		if len(args) != 1 {
			return ctx, false, errors.New("invalid params")
		}
		hash, ok := args[0].(common.H256)
		if !ok {
			return ctx, false, errors.New("invalid params")
		}
		res, err := l.client.TraceTransaction(hash)
		if err != nil {
			return ctx, true, err
		}
		*response = res
		return ctx, true, nil

	case "eth_estimateGas":
		if len(args) != 2 {
			return ctx, false, errors.New("invalid params")
		}
		tx, ok := args[0].(engine.TransactionForCall)
		if !ok {
			return ctx, false, errors.New("invalid params")
		}
		number, ok := args[1].(*common.BN64)
		if !ok {
			return ctx, false, errors.New("invalid params")
		}
		res, err := l.client.EstimateGas(tx, number)
		if err != nil {
			return ctx, true, err
		}
		*response = res
		return ctx, true, nil

	default:
		return ctx, false, nil
	}
}

// Post implements endpoint.Processor.
func (*LocalProxy) Post(ctx context.Context, _ string, _ *any, _ *error) context.Context {
	return ctx
}

type rpcClient struct {
	conn    net.Conn
	timeout time.Duration
}

func newRPCClient(address string, timeout time.Duration) (*rpcClient, error) {
	c, err := net.Dial("unix", address)
	if err != nil {
		return nil, err
	}
	return &rpcClient{c, timeout}, nil
}

func (rc *rpcClient) Close() error {
	return rc.conn.Close()
}

func (rc *rpcClient) TraceTransaction(hash common.H256) (*response.CallFrame, error) {
	req, err := buildRequest("debug_traceTransaction", hash)
	if err != nil {
		return nil, err
	}

	res, err := rc.request(req)
	if err != nil {
		return nil, err
	}

	result, resultType, _, err := jsonparser.Get(res, "result")
	if err != nil {
		return nil, err
	}

	switch resultType {
	case jsonparser.NotExist:
		rpcErr, rpcErrType, _, err := jsonparser.Get(res, "error", "message")
		switch {
		case err != nil:
			return nil, err
		case rpcErrType == jsonparser.NotExist:
			return nil, errors.New("internal rpc error")
		default:
			return nil, fmt.Errorf("%s", rpcErr)
		}

	case jsonparser.Object:
		trace := new(response.CallFrame)
		err := json.Unmarshal(result, trace)
		return trace, err

	case jsonparser.Array:
		traces := make([]*response.CallFrame, 0, 1)
		_, err := jsonparser.ArrayEach(result, func(value []byte, dataType jsonparser.ValueType, _ int, _ error) {
			trace := new(response.CallFrame)
			err = json.Unmarshal(value, trace)
			if err != nil {
				return
			}
			traces = append(traces, trace)
		})
		if len(traces) != 1 {
			return nil, errors.New("unexpected response")
		}
		return traces[0], err

	default:
		return nil, errors.New("failed to parse unexpected response")
	}
}

func (rc *rpcClient) EstimateGas(tx engine.TransactionForCall, number *common.BN64) (*common.Uint256, error) {
	req, err := buildRequest("eth_estimateGas", tx, number)
	if err != nil {
		return nil, err
	}

	res, err := rc.request(req)
	if err != nil {
		return nil, err
	}

	result, resultType, _, err := jsonparser.Get(res, "result")
	if err != nil {
		return nil, err
	}

	switch resultType {
	case jsonparser.NotExist:
		rpcErr, rpcErrType, _, err := jsonparser.Get(res, "error", "message")
		switch {
		case err != nil:
			return nil, err
		case rpcErrType == jsonparser.NotExist:
			return nil, errors.New("internal rpc error")
		default:
			return nil, fmt.Errorf("%s", rpcErr)
		}

	case jsonparser.String:
		resp := new(common.Uint256)
		err := json.Unmarshal(result, resp)
		return resp, err

	default:
		return nil, errors.New("failed to parse unexpected response")
	}
}

func (rc *rpcClient) request(req []byte) ([]byte, error) {
	err := rc.conn.SetDeadline(time.Now().Add(rc.timeout))
	if err != nil {
		return nil, err
	}

	err = binary.Write(rc.conn, binary.LittleEndian, uint32(len(req)))
	if err != nil {
		return nil, err
	}

	_, err = rc.conn.Write(req)
	if err != nil {
		return nil, err
	}

	var l uint32
	err = binary.Read(rc.conn, binary.LittleEndian, &l)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, l)
	_, err = io.ReadFull(rc.conn, buf)
	return buf, err
}

func buildRequest(method string, params ...any) ([]byte, error) {
	b := bytes.NewBufferString(`{"id":1,"jsonrpc":"2.0","method":"` + method + `","params":[`)
	first := true
	for _, param := range params {
		p, err := json.Marshal(param)
		if err != nil {
			return nil, err
		}

		if !first {
			b.WriteByte(',')
		}
		first = false

		b.Write(p)
	}
	b.WriteString("]}")
	return b.Bytes(), nil
}
