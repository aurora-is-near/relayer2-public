package indexer

import (
	"aurora-relayer-go-common/db"
	"aurora-relayer-go-common/log"
	"errors"
)

type JsonFileIndexer struct {
	dbHandler *db.Handler
	logger    *log.Logger
	Config    *Config
}

func New(dbh db.Handler) (*JsonFileIndexer, error) {
	if dbh == nil {
		return nil, errors.New("db handler is not initialized")
	}

	logger := log.Log()
	config := GetConfig()

	// TODO implement indexer

	return &JsonFileIndexer{
		dbHandler: &dbh,
		logger:    logger,
		Config:    config,
	}, nil
}

func (j JsonFileIndexer) Close() error {
	// TODO implement me
	return nil
}
