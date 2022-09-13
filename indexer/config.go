package indexer

import (
	"aurora-relayer-go-common/log"
	"github.com/spf13/viper"
)

const (
	GenesisBlock = 9820210

	DefaultKeepFiles           = false
	DefaultFromBlock           = GenesisBlock
	DefaultToBlock             = 0
	DefaultSubFolderBatchSize  = 10000
	DefaultRetryCountOnFailure = 10
	DefaultWaitForBlockMs      = 500
	DefaultSourceFolder        = "/tmp/relayer/json/"

	configPath = "indexer"
)

type Config struct {
	KeepFiles           bool   `mapstructure:"keepFiles"`
	RetryCountOnFailure uint8  `mapstructure:"retryCountOnFailure"`
	SubFolderBatchSize  uint16 `mapstructure:"subFolderBatchSize"`
	FromBlock           uint64 `mapstructure:"fromBlock"`
	ToBlock             uint64 `mapstructure:"toBlock"`
	SourceFolder        string `mapstructure:"sourceFolder"`
	WaitForBlockMs      uint16 `mapstructure:"waitForBlockMs"`
}

func defaultConfig() *Config {
	return &Config{
		KeepFiles:           DefaultKeepFiles,
		RetryCountOnFailure: DefaultRetryCountOnFailure,
		FromBlock:           DefaultFromBlock, // inclusive
		ToBlock:             DefaultToBlock,   // exclusive
		SubFolderBatchSize:  DefaultSubFolderBatchSize,
		SourceFolder:        DefaultSourceFolder,
		WaitForBlockMs:      DefaultWaitForBlockMs,
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
