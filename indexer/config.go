package indexer

import (
	"aurora-relayer-go-common/log"
	"github.com/spf13/viper"
)

const (
	DefaultJsonFilesPath = "/tmp/relayer/json/"

	configPath = "indexer"
)

type Config struct {
	JsonFilesPath string `mapstructure:"jsonFilesPath"`
}

func defaultConfig() *Config {
	return &Config{
		JsonFilesPath: DefaultJsonFilesPath,
	}
}

func GetConfig() *Config {
	config := defaultConfig()
	sub := viper.Sub(configPath)
	if sub != nil {
		if err := sub.Unmarshal(&config); err != nil {
			log.Log().Warn().Err(err).Msgf("failed to parse configuration [%s] from [%s], "+
				"falling back to defaults", configPath, viper.ConfigFileUsed())
		}
	}
	return config
}
