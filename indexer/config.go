package indexer

import (
	"relayer2-base/cmd"
	"relayer2-base/log"

	"github.com/spf13/viper"
)

const (
	DefaultGenesisBlock        = 9820210
	DefaultKeepFiles           = false
	DefaultForceReindex        = false
	DefaultFromBlock           = DefaultGenesisBlock
	DefaultToBlock             = 0
	DefaultSubFolderBatchSize  = 10000
	DefaultRetryCountOnFailure = 10
	DefaultWaitForBlockMs      = 500
	DefaultSourceFolder        = "/tmp/relayer/json/"

	configPath = "indexer"
)

type Config struct {
	KeepFiles           bool   `mapstructure:"keepFiles"`
	ForceReindex        bool   `mapstructure:"forceReindex"`
	RetryCountOnFailure uint8  `mapstructure:"retryCountOnFailure"`
	SubFolderBatchSize  uint16 `mapstructure:"subFolderBatchSize"`
	GenesisBlock        uint64 `mapstructure:"genesisBlock"`
	FromBlock           uint64 `mapstructure:"fromBlock"`
	ToBlock             uint64 `mapstructure:"toBlock"`
	SourceFolder        string `mapstructure:"sourceFolder"`
	WaitForBlockMs      uint16 `mapstructure:"waitForBlockMs"`
}

func defaultConfig() *Config {
	return &Config{
		KeepFiles:           DefaultKeepFiles,
		ForceReindex:        DefaultForceReindex,
		RetryCountOnFailure: DefaultRetryCountOnFailure,
		GenesisBlock:        DefaultGenesisBlock,
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
		cmd.BindSubViper(sub, configPath)
		if err := sub.Unmarshal(&config); err != nil {
			log.Log().Warn().Err(err).Msgf("failed to parse configuration [%s] from [%s], "+
				"falling back to defaults", configPath, viper.ConfigFileUsed())
		}
	}

	return config
}
