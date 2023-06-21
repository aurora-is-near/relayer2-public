package middleware

import (
	"context"
	"io/fs"
	"net"

	"github.com/aurora-is-near/relayer2-base/log"
	"github.com/aurora-is-near/relayer2-base/rpc"
	errs "github.com/aurora-is-near/relayer2-base/types/errors"
	"github.com/aurora-is-near/relayer2-base/utils"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

const (
	filterConfigPath      = "endpoint.filterFilePath"
	defaultFilterFilePath = "config/filter.yaml"
)

type filter struct {
	policy string
	list   []*net.IPNet
}

func FilterIP() rpc.Middleware {
	// use global viper to get filter config file path
	filterFile := viper.GetString(filterConfigPath)
	if filterFile == "" {
		filterFile = defaultFilterFilePath
	}

	// not using global viper, see;
	// https://github.com/spf13/viper/issues/442
	// https://github.com/spf13/viper/issues/631
	v := viper.New()
	v.SetConfigFile(filterFile)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(*fs.PathError); !ok {
			log.Log().Fatal().Err(err).Msg("failed to read config file filter.yml")
		} else {
			log.Log().Warn().Msg("config file filter.yml not found")
		}
	} else {
		v.WatchConfig()
	}

	var f filter
	setConfig(&f, v)
	v.OnConfigChange(func(e fsnotify.Event) {
		setConfig(&f, v)
	})

	return func(handler rpc.RpcHandler) rpc.RpcHandler {
		return func(ctx *context.Context, rpcCtx *rpc.RpcContext) *rpc.RpcContext {
			if len(f.list) > 0 {

				clientIp, supported := utils.ClientIpFromContext(*ctx)
				if !supported || clientIp == nil {
					return rpcCtx.SetErrorObject(&errs.InternalError{Message: "unknown client"})
				}
				if f.policy == "allow" {
					for _, ipNet := range f.list {
						if ipNet.Contains(*clientIp) {
							return handler(ctx, rpcCtx)
						}
					}
					log.Log().Warn().Msgf("IP [%s] is in blacklist", clientIp)
					return rpcCtx.SetErrorObject(&errs.InternalError{Message: "forbidden client"})
				} else {
					for _, ipNet := range f.list {
						if ipNet.Contains(*clientIp) {
							log.Log().Warn().Msgf("IP [%s] is in blacklist", clientIp)
							return rpcCtx.SetErrorObject(&errs.InternalError{Message: "forbidden client"})
						}
					}
				}
			}
			return handler(ctx, rpcCtx)
		}
	}
}

// setConfig sets provided configuration to the provided filter
func setConfig(f *filter, v *viper.Viper) {
	p := v.GetString("filter.IP.policy")
	l := v.GetStringSlice("filter.IP.list")

	if p == "allow" {
		f.policy = p
	} else {
		f.policy = "deny"
	}

	f.list = make([]*net.IPNet, 0, len(l))

	for _, cidr := range l {
		if ip := net.ParseIP(cidr); ip != nil {
			cidr = cidr + "/32"
		}
		if _, ipNet, err := net.ParseCIDR(cidr); err != nil {
			log.Log().Err(err).Msgf("failed parsing IP filtering config for [%s]", cidr)
			continue
		} else {
			f.list = append(f.list, ipNet)
		}
	}
}
