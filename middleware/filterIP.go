package middleware

import (
	"github.com/aurora-is-near/relayer2-base/log"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"io/fs"
	"net"
	"net/http"
	"strings"
)

const (
	configPath            = "endpoint.filterFilePath"
	defaultFilterFilePath = "config/filter.yaml"
)

type filter struct {
	policy string
	list   []*net.IPNet
}

func FilterIP(next http.Handler) http.Handler {

	// use global viper to get filter config file path
	filterFile := viper.GetString(configPath)
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(f.list) > 0 {
			ip, err := getSourceIP(r)
			if err != nil || ip == nil { // not likely but possible
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			if f.policy == "allow" {
				for _, ipNet := range f.list {
					if ipNet.Contains(ip) {
						next.ServeHTTP(w, r)
						return
					}
				}
				log.Log().Warn().Msgf("IP [%s] is in blacklist", ip)
				http.Error(w, "", http.StatusForbidden)
				return
			} else {
				for _, ipNet := range f.list {
					if ipNet.Contains(ip) {
						log.Log().Warn().Msgf("IP [%s] is in blacklist", ip)
						http.Error(w, "", http.StatusForbidden)
						return
					}
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func getSourceIP(r *http.Request) (net.IP, error) {

	// Check the Forwarded header
	forwardedHeader := r.Header.Get("Forwarded")
	if forwardedHeader != "" {
		parts := strings.Split(forwardedHeader, ",")
		firstPart := strings.TrimSpace(parts[0])
		subParts := strings.Split(firstPart, ";")
		for _, part := range subParts {
			normalisedPart := strings.ToLower(strings.TrimSpace(part))
			if strings.HasPrefix(normalisedPart, "for=") {
				return net.ParseIP(normalisedPart[4:]), nil
			}
		}
	}

	// Check the X-Forwarded-For header
	xForwardedForHeader := r.Header.Get("X-Forwarded-For")
	if xForwardedForHeader != "" {
		parts := strings.Split(xForwardedForHeader, ",")
		lastPart := strings.TrimSpace(parts[len(parts)-1])
		return net.ParseIP(lastPart), nil
	}

	// Check on the request
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil, err
	}

	return net.ParseIP(host), nil
}

func setConfig(f *filter, v *viper.Viper) {
	p := v.GetString("filter.IP.policy")
	l := v.GetStringSlice("filter.IP.list")

	if "allow" == p {
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
