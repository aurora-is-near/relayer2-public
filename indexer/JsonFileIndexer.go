package indexer

import (
	"aurora-relayer-go-common/db"
	"aurora-relayer-go-common/log"
	"github.com/spf13/viper"
)

const (
	configPath = "Indexer.JsonFileIndexer"
)

type JsonFileIndexer struct {
	dbHandler *db.Handler
	logger    *log.Logger
}

func New(dbh db.Handler) *JsonFileIndexer {
	if dbh == nil {
		panic("DB Handler should be initialized")
	}

	logger := log.Log()
	conf := DefaultConfig()
	sub := viper.Sub(configPath)
	if sub != nil {
		if err := sub.Unmarshal(&conf); err != nil {
			logger.Warn().Err(err).Msgf("failed to parse configuration [%s] from [%s], "+
				"falling back to defaults", configPath, viper.ConfigFileUsed())
		}
	}

	return &JsonFileIndexer{
		dbHandler: &dbh,
		logger:    logger,
	}
}

func (j JsonFileIndexer) Start() error {
	// TODO implement me

	// panic("implement me")
	return nil
}

func (j JsonFileIndexer) Stop() error {
	// TODO implement me

	// panic("implement me")
	return nil
}
