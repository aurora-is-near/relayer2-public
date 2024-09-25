package indexer

import (
	"context"
	"errors"
	"fmt"

	"github.com/aurora-is-near/relayer2-base/broker"
	"github.com/aurora-is-near/relayer2-base/cmdutils"
	"github.com/aurora-is-near/relayer2-base/db"
	"github.com/aurora-is-near/relayer2-base/log"

	"github.com/spf13/viper"
)

const (
	SourceRefiner   = "refiner"
	SourceBlocksApi = "blocksApi"

	DefaultSource              = SourceRefiner
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
	Source       string `mapstructure:"source"`
	GenesisBlock uint64 `mapstructure:"genesisBlock"`
	FromBlock    uint64 `mapstructure:"fromBlock"`
	ToBlock      uint64 `mapstructure:"toBlock"`
	ForceReindex bool   `mapstructure:"forceReindex"`

	// Refiner source specific
	KeepFiles           bool   `mapstructure:"keepFiles"`
	SourceFolder        string `mapstructure:"sourceFolder"`
	SubFolderBatchSize  uint16 `mapstructure:"subFolderBatchSize"`
	RetryCountOnFailure uint8  `mapstructure:"retryCountOnFailure"`
	WaitForBlockMs      uint16 `mapstructure:"waitForBlockMs"`

	// BlocksApi source specific
	BlocksApiUrl   []string `mapstructure:"blocksApiUrl"`
	BlocksApiStream string   `mapstructure:"blocApiToken"`
	BlocksApiToken string   `mapstructure:"blocApiToken"`
}

func defaultConfig() *Config {
	return &Config{
		Source:       DefaultSource,
		GenesisBlock: DefaultGenesisBlock,
		FromBlock:    DefaultFromBlock, // inclusive
		ToBlock:      DefaultToBlock,   // exclusive
		ForceReindex: DefaultForceReindex,

		RetryCountOnFailure: DefaultRetryCountOnFailure,
		WaitForBlockMs:      DefaultWaitForBlockMs,

		KeepFiles:          DefaultKeepFiles,
		SubFolderBatchSize: DefaultSubFolderBatchSize,
		SourceFolder:       DefaultSourceFolder,
	}
}

func GetConfig() *Config {
	config := defaultConfig()
	sub := viper.Sub(configPath)
	if sub != nil {
		cmdutils.BindSubViper(sub, configPath)
		if err := sub.Unmarshal(&config); err != nil {
			log.Log().Warn().Err(err).Msgf("failed to parse configuration [%s] from [%s], "+
				"falling back to defaults", configPath, viper.ConfigFileUsed())
		}
	}

	return config
}

type Indexer interface {
	Start(ctx context.Context)
	Close()
}

func New(dbh db.Handler, b broker.Broker) (Indexer, error) {
	if dbh == nil {
		return nil, errors.New("db handler is not initialized")
	}

	config := GetConfig()
	logger := log.Log()

	// validate & normalise config
	fromBlockUpdated := false

	if !config.ForceReindex {
		lb, err := dbh.BlockNumber(context.Background())
		if err == nil && lb != nil {
			bn := uint64(*lb)
			logger.Info().Msgf("latest indexed block: [%d]", bn)

			// fromBlock should not be updated for pre history blocks
			if bn >= config.GenesisBlock {
				logger.Warn().Msgf("overwriting fromBlock: [%d] as [%d]", config.FromBlock, bn+1)

				config.FromBlock = bn + 1
				fromBlockUpdated = true
			}
		}
	}

	if (config.ToBlock > DefaultToBlock) && (config.ToBlock <= config.FromBlock) {
		if fromBlockUpdated && (config.FromBlock < config.ToBlock) {
			return nil, fmt.Errorf("given block range is already indexed till [%d]. Please either enable `forceReindex` config to re-index or update the range and re-start the application", config.FromBlock)
		} else {
			return nil, fmt.Errorf("invalid config, toBlock: [%d] must be greater than fromBlock: [%d]", config.ToBlock, config.FromBlock)
		}
	}

	switch config.Source {
	case SourceRefiner:
		return NewIndexerRefiner(config, dbh, b)

	case SourceBlocksApi:
		return NewIndexerBlocksApi(config, dbh, b)

	default:
		return nil, fmt.Errorf("invalid config, source: the source must be %s or %s", SourceRefiner, SourceBlocksApi)
	}
}
