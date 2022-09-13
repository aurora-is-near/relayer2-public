package indexer

import (
	"aurora-relayer-go-common/db"
	"aurora-relayer-go-common/log"
	"aurora-relayer-go-common/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type processIndexerState func(*Indexer) processIndexerState

type indexerStats struct {
	totalBlocksIndexed          uint64
	avgBlockIndexingRateSeconds float32 // TODO add (EW) Moving Avg
	blockIndexingStartTime      time.Time
}

type indexerState struct {
	currBlock    uint64
	subBlock     uint64
	batchSize    uint64
	filePath     string
	subBlockPath string
	retryCount   uint8
	started      bool
	block        *utils.Block
	stats        *indexerStats
}

type Indexer struct {
	dbh    *db.Handler
	l      *log.Logger
	c      *Config
	s      *indexerState
	lock   *sync.Mutex
	stopCh chan bool
}

// New creates the indexer and starts the indexer state machine
func New(dbh db.Handler) (*Indexer, error) {
	if dbh == nil {
		return nil, errors.New("db handler is not initialized")
	}

	logger := log.Log()
	config := GetConfig()

	var fromBlock uint64
	lb, err := dbh.BlockNumber()
	if err != nil {
		fromBlock = GenesisBlock
	} else {
		fromBlock = lb.Uint64()
		logger.Info().Msgf("latest indexed block: [%d]", fromBlock)
		fromBlock += 1
	}
	if config.FromBlock < fromBlock {
		logger.Warn().Msgf("overwriting fromBlock: [%d] as [%d]", config.FromBlock, fromBlock)
		config.FromBlock = fromBlock
	}
	if (config.ToBlock > DefaultToBlock) && (config.ToBlock <= config.FromBlock) {
		err := fmt.Errorf("invalid config, toBlock: [%d] must be greater than fromBlock: [%d]", config.ToBlock, config.FromBlock)
		return nil, err
	}

	bs := uint64(config.SubFolderBatchSize)
	sb := config.FromBlock / bs * bs
	sbp := filepath.Join(config.SourceFolder, fmt.Sprintf("%v", sb))
	fp := filepath.Join(sbp, fmt.Sprintf("%v.json", config.FromBlock))
	indexer := &Indexer{
		dbh: &dbh,
		l:   logger,
		c:   config,
		s: &indexerState{
			currBlock:    config.FromBlock,
			batchSize:    bs,
			subBlock:     sb,
			subBlockPath: sbp,
			filePath:     fp,
			retryCount:   uint8(0),
			block:        nil,
			stats: &indexerStats{
				totalBlocksIndexed:          uint64(0),
				avgBlockIndexingRateSeconds: float32(0),
				blockIndexingStartTime:      time.Now(),
			},
		},
		lock:   &sync.Mutex{},
		stopCh: make(chan bool),
	}

	return indexer, nil
}

func (i *Indexer) Start() {
	i.lock.Lock()
	defer i.lock.Unlock()
	if !i.s.started {
		i.s.started = true
		i.s.stats.blockIndexingStartTime = time.Now()
		go i.index()
	}
}

// Close gracefully stops indexer state machine
func (i *Indexer) Close() {
	i.lock.Lock()
	defer i.lock.Unlock()
	if i.s.started {
		stop(i)
		i.s.started = false
	}
}

// index starts the indexer state machine
func (i *Indexer) index() {
	i.l.Info().Msgf("starting indexing fromBlock: [%d], source: [%s]", i.c.FromBlock, i.c.SourceFolder)
	f := read
	for {
		f = f(i)
		select {
		case <-i.stopCh:
			return
		default:
		}
	}
}

// evalError should be called by processIndexerState typed functions for all errors, returns;
// 	nil, if err is nil. i.e.: no state change required, hence caller should continue
// 	onRetry processIndexerState, if err is not nil and max retry count not exceeded
// 	onFail processIndexerState, if err is not nil and max retry count is exceeded
func (i *Indexer) evalError(err error, onRetry processIndexerState, onFail processIndexerState) processIndexerState {
	if err == nil {
		return nil
	}
	if i.s.retryCount < i.c.RetryCountOnFailure {
		i.l.Error().Err(err).Msgf("retrying block: [%d] on failure", i.s.currBlock)
		i.s.retryCount += 1
		time.Sleep(time.Duration(i.c.WaitForBlockMs) * time.Millisecond)
		return onRetry
	}
	i.l.Error().Err(err).Msgf("exceeded max onRetry count: [%d] for block: [%d]", i.c.RetryCountOnFailure, i.s.currBlock)
	return onFail
}

// reads block data from file pointed by indexerState.filePath and parses file content into utils.Block. If file does not
// exit on the specified path, it retries/waits indefinitely
// 	On success, continues with insert
// 	On failure, retries for Config.RetryCountOnFailure and if all retries fails then stops indexer
func read(i *Indexer) processIndexerState {
	i.l.Debug().Msgf("reading block: [%d], path: [%s]", i.s.currBlock, i.s.filePath)
	buff, err := os.ReadFile(i.s.filePath)
	if os.IsNotExist(err) {
		time.Sleep(time.Duration(i.c.WaitForBlockMs) * time.Millisecond)
		return read
	}
	if next := i.evalError(err, read, stop); next != nil {
		return next
	}
	err = json.Unmarshal(buff, &i.s.block)
	if next := i.evalError(err, read, stop); next != nil {
		return next
	}
	return insert
}

// insert adds the block to persistent storage.
// 	On success, continues with either increment or removeFile.
// 	On failure, retries for Config.RetryCountOnFailure and if all retries fails then stops indexer
func insert(i *Indexer) processIndexerState {
	i.l.Debug().Msgf("inserting block: [%d]", i.s.block.Height)
	i.s.block.Sequence = utils.UintToUint256(i.s.block.Height)
	err := (*i.dbh).InsertBlock(i.s.block)
	if next := i.evalError(err, insert, stop); next != nil {
		return next
	}
	if i.c.KeepFiles {
		return increment
	} else {
		return removeFile
	}
}

// removeFile tries to remove file pointed by indexerState.filePath. If the file is the last file of the sub block it
// continues with removeFolder state otherwise increment to next block. This is a best-effort operation if there is a
// failure during remove operation, it retries for Config.RetryCountOnFailure and continues with the next block in any case.
func removeFile(i *Indexer) processIndexerState {
	i.l.Debug().Msgf("removing file: [%s]", i.s.filePath)
	err := os.Remove(i.s.filePath)
	if next := i.evalError(err, removeFile, increment); next != nil {
		return next
	}
	// if next block is equal to next sub block and current sub block dir is empty
	if ((i.s.currBlock + 1) == (i.s.subBlock + i.s.batchSize)) && isDirEmpty(i.s.subBlockPath) {
		return removeFolder
	}
	return increment
}

// removeFolder tries to remove the directory pointed by indexerState.subBlockPath. This is a best-effort operation if
// there is a failure during remove operation, it retries for Config.RetryCountOnFailure and continues with the next
// block in any case.
func removeFolder(i *Indexer) processIndexerState {
	i.l.Debug().Msgf("removing folder: [%s]", i.s.subBlockPath)
	err := os.Remove(i.s.subBlockPath)
	if next := i.evalError(err, removeFolder, increment); next != nil {
		return next
	}
	return increment
}

// increment prepares the indexerState for next block processing and sets indexerStats, returns;
//	stop processIndexerState, if Config.ToBlock is specified and reached
//	read processIndexerState, otherwise
func increment(i *Indexer) processIndexerState {
	i.s.stats.totalBlocksIndexed += 1
	if (i.c.ToBlock > 0) && (i.c.ToBlock <= i.s.currBlock) {
		i.l.Info().Msgf("indexing finished fromBlock: [%d], toBlock: [%d]", i.c.FromBlock, i.c.ToBlock)
		return stop
	}
	i.s.currBlock += 1
	i.s.subBlock = i.s.currBlock / i.s.batchSize * i.s.batchSize
	i.s.retryCount = 0
	i.s.block = nil
	i.s.subBlockPath = filepath.Join(i.c.SourceFolder, fmt.Sprintf("%v", i.s.subBlock))
	i.s.filePath = filepath.Join(i.s.subBlockPath, fmt.Sprintf("%v.json", i.s.currBlock))
	i.s.stats.blockIndexingStartTime = time.Now()
	i.l.Info().Msgf("indexing block: [%d] from: [%s]", i.s.currBlock, i.s.filePath)
	return read
}

// stops indexer state machine.
func stop(i *Indexer) processIndexerState {
	lastBlock, err := (*i.dbh).BlockNumber()
	if err != nil {
		i.l.Error().Err(err).Msg("failed to get last indexed block")
	}
	i.l.Info().Msgf("stopping indexer, last indexed block: [%v], current processed block: [%d]", lastBlock, i.s.currBlock)
	i.stopCh <- true
	return nil
}

// TODO: state to filter/publish subscription data
func publish(i *Indexer) processIndexerState {
	return nil
}

// isDirEmpty checks whether a directory is empty or not, returns;
// 	false if directory is not empty or fails to get dir info
// 	true otherwise
func isDirEmpty(dirName string) bool {
	files, err := ioutil.ReadDir(dirName)
	if err != nil || len(files) != 0 {
		return false
	}
	return true
}
