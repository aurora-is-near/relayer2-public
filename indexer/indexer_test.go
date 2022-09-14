package indexer

import (
	"aurora-relayer-go-common/db"
	"aurora-relayer-go-common/db/badger"
	"aurora-relayer-go-common/utils"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const testDir = "test"

const toBlockCannotBeGreaterThanFromBlockYml = `
db:
  badger:
      gcIntervalSeconds: 1
      iterationTimeoutSeconds: 5
      iterationMaxItems: 10000
      logFilterTtlMinutes: 15
      index:
        maxJumps: 1000
        maxRangeScanners: 2
        maxValueFetchers: 2
        keysOnly: false
      options:
        InMemory: true
        DetectConflicts: true
indexer:
  sourceFolder: "test"
  subFolderBatchSize: 10000
  keepFiles: true
  toBlock: 10
  fromBlock: 100
`

const fromBlockCannotBeSmallerThanGenesisYaml = `
db:
  badger:
      gcIntervalSeconds: 1
      iterationTimeoutSeconds: 5
      iterationMaxItems: 10000
      logFilterTtlMinutes: 15
      index:
        maxJumps: 1000
        maxRangeScanners: 2
        maxValueFetchers: 2
        keysOnly: false
      options:
        InMemory: true
        DetectConflicts: true
indexer:
  sourceFolder: "test"
  subFolderBatchSize: 10000
  keepFiles: true
  fromBlock: 100
`

const waitForFileIndefinitelyYml = `
db:
  badger:
      gcIntervalSeconds: 1
      iterationTimeoutSeconds: 5
      iterationMaxItems: 10000
      logFilterTtlMinutes: 15
      index:
        maxJumps: 1000
        maxRangeScanners: 2
        maxValueFetchers: 2
        keysOnly: false
      options:
        InMemory: true
        DetectConflicts: true
indexer:
  sourceFolder: "test"
  subFolderBatchSize: 10000
  keepFiles: true
  fromBlock: 9820250
  retryCountOnFailure: 3
`

const stopsAfterMaxRetryYml = `
db:
  badger:
      gcIntervalSeconds: 1
      iterationTimeoutSeconds: 5
      iterationMaxItems: 10000
      logFilterTtlMinutes: 15
      index:
        maxJumps: 1000
        maxRangeScanners: 2
        maxValueFetchers: 2
        keysOnly: false
      options:
        InMemory: true
        DetectConflicts: true
indexer:
  sourceFolder: "test"
  subFolderBatchSize: 10000
  keepFiles: true
  retryCountOnFailure: 3
`

const removeFilesFoldersYml = `
db:
  badger:
      gcIntervalSeconds: 1
      iterationTimeoutSeconds: 5
      iterationMaxItems: 10000
      logFilterTtlMinutes: 15
      index:
        maxJumps: 1000
        maxRangeScanners: 2
        maxValueFetchers: 2
        keysOnly: false
      options:
        InMemory: true
        DetectConflicts: true
indexer:
  sourceFolder: "test"
  subFolderBatchSize: 10000
  keepFiles: false
  retryCountOnFailure: 3
`

const withToAndFromBlockYml = `
db:
  badger:
      gcIntervalSeconds: 1
      iterationTimeoutSeconds: 5
      iterationMaxItems: 10000
      logFilterTtlMinutes: 15
      index:
        maxJumps: 1000
        maxRangeScanners: 2
        maxValueFetchers: 2
        keysOnly: false
      options:
        InMemory: true
        DetectConflicts: true
indexer:
  sourceFolder: "test"
  subFolderBatchSize: 10000
  keepFiles: false
  retryCountOnFailure: 3
  toBlock: 9820211
  fromBlock: 9820210
`

var blocks = []string{`
{
  "chain_id": 1313161554,
  "hash": "0xa2a07fe210b15810f28c3010cf8702771ba80e014f2a5be3106c774b19f66bea",
  "parent_hash": "0x45030fddc1d668519df3495fad83e1844ca9a805f1a08190f1c44e4d4bda71e9",
  "height": 9820211,
  "miner": "0xdcc703c0e500b653ca82273b7bfad8045d85a470",
  "timestamp": 1595368210762782796,
  "gas_limit": "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
  "gas_used": "0x0",
  "logs_bloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
  "size": "0x0",
  "transactions_root": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
  "state_root": "0x473879d6725b890240eee186aea1f14960e2132094d55ba70195819a7a155372",
  "receipts_root": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
  "transactions": [
    
  ],
  "near_metadata": "SkipBlock"
}`, `
{
  "chain_id": 1313161554,
  "hash": "0x45030fddc1d668519df3495fad83e1844ca9a805f1a08190f1c44e4d4bda71e9",
  "parent_hash": "0x54020a3a8fd8ddf0be6b460652e2a4acbeb62ef998b373223a970167f9151b1c",
  "height": 9820210,
  "miner": "0x06c761d6af68d23a2872f0e89f895811e60565fd",
  "timestamp": 1595350551591948000,
  "gas_limit": "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
  "gas_used": "0x0",
  "logs_bloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
  "size": "0x0",
  "transactions_root": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
  "state_root": "0x473879d6725b890240eee186aea1f14960e2132094d55ba70195819a7a155372",
  "receipts_root": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
  "transactions": [
    
  ],
  "near_metadata": {
    "ExistingBlock": {
      "near_hash": "EPnLgE7iEq9s7yTkos96M3cWymH5avBAPm3qx3NXqR8H",
      "near_parent_hash": "11111111111111111111111111111111",
      "author": "nfvalidator1.near"
    }
  }
}`,
}

type TestItem struct {
	name        string
	enabled     bool
	config      string
	failMsg     string
	setup       func(args ...interface{})
	teardown    func(args ...interface{})
	call        func(args ...interface{}) (interface{}, error)
	args        []interface{}
	want        interface{}
	errContains string
}

func TestConfiguration(t *testing.T) {

	var sh db.StoreHandler

	tests := []TestItem{
		{
			name:    "fromBlock cannot be smaller than genesis",
			enabled: true,
			config:  fromBlockCannotBeSmallerThanGenesisYaml,
			failMsg: "fromBlock should be equal to Genesis Block",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(fromBlockCannotBeSmallerThanGenesisYaml)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				return i.c.FromBlock, err
			},
			want:        uint64(GenesisBlock),
			errContains: "",
		},
		{
			name:    "toBlock cannot be greater than fromBlock",
			enabled: true,
			config:  toBlockCannotBeGreaterThanFromBlockYml,
			failMsg: "should have returned nil with invalid config",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(toBlockCannotBeGreaterThanFromBlockYml)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
			},
			call: func(args ...interface{}) (interface{}, error) {
				return New(args[0].(*db.StoreHandler))
			},
			want:        nil,
			errContains: "invalid config",
		},
		{
			name:    "fromBlock cannot be smaller than latest indexed block",
			enabled: true,
			config:  fromBlockCannotBeSmallerThanGenesisYaml,
			failMsg: "fromBlock should be equal latest indexed block",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(fromBlockCannotBeSmallerThanGenesisYaml)
				args[0] = initStoreHandler()
				insertBlocks(args[0].(*db.StoreHandler))
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				return i.c.FromBlock, err
			},
			want:        uint64(GenesisBlock+3) + 1,
			errContains: "",
		},
	}

	runTests(tests, t)
}

func TestStateTransitions(t *testing.T) {

	var sh db.StoreHandler

	tests := []TestItem{
		{
			name:    "read waits indefinitely for new file",
			enabled: true,
			config:  waitForFileIndefinitelyYml,
			failMsg: "read should wait indefinitely for new block file",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(waitForFileIndefinitelyYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				r := read
				for n := uint8(0); n < i.c.RetryCountOnFailure+1; n++ {
					r = r(i)
				}
				return nameOfFunc(r), err
			},
			want:        nameOfFunc(read),
			errContains: "",
		},
		{
			name:    "read stops if retries exceeds RetryCountOnFailure",
			enabled: true,
			config:  stopsAfterMaxRetryYml,
			failMsg: "read should stop after max retry if err type is different than 'file not exists'",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(stopsAfterMaxRetryYml)
				createFiles(blocks, 5)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				r := read
				for n := uint8(0); n < i.c.RetryCountOnFailure+1; n++ {
					r = r(i)
				}
				return nameOfFunc(r), err
			},
			want:        nameOfFunc(stop),
			errContains: "",
		},
		{
			name:    "read to insert on success",
			enabled: true,
			config:  stopsAfterMaxRetryYml,
			failMsg: "read should return insert on success",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(stopsAfterMaxRetryYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				return nameOfFunc(read(i)), err
			},
			want:        nameOfFunc(insert),
			errContains: "",
		},
		{
			name:    "insert to publish on success",
			enabled: true,
			config:  stopsAfterMaxRetryYml,
			failMsg: "insert should return insert on success if keepFiles is true",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(stopsAfterMaxRetryYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				r := read(i)
				r = r(i)
				return nameOfFunc(r), err
			},
			want:        nameOfFunc(publish),
			errContains: "",
		},
		{
			name:    "publish to increment on success",
			enabled: true,
			config:  stopsAfterMaxRetryYml,
			failMsg: "insert should return insert on success if keepFiles is true",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(stopsAfterMaxRetryYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				r := read(i)
				r = r(i)
				r = r(i)
				return nameOfFunc(r), err
			},
			want:        nameOfFunc(increment),
			errContains: "",
		},
		{
			name:    "publish to removeFile on success",
			enabled: true,
			config:  removeFilesFoldersYml,
			failMsg: "insert should return removeFile on success if keepFiles is true",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(removeFilesFoldersYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				r := read(i)
				r = r(i)
				r = r(i)
				return nameOfFunc(r), err
			},
			want:        nameOfFunc(removeFile),
			errContains: "",
		},
		{
			name:    "removeFile to increment if retries exceeds RetryCountOnFailure",
			enabled: true,
			config:  removeFilesFoldersYml,
			failMsg: "removeFile should return increment on failure",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(removeFilesFoldersYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				i.s.filePath = "invalid"
				r := removeFile
				for n := uint8(0); n < i.c.RetryCountOnFailure+1; n++ {
					r = r(i)
				}
				return nameOfFunc(r), err
			},
			want:        nameOfFunc(increment),
			errContains: "",
		},
		{
			name:    "removeFile to increment on success and case0",
			enabled: true,
			config:  removeFilesFoldersYml,
			failMsg: "removeFile should return increment on success",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(removeFilesFoldersYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				return nameOfFunc(removeFile(i)), err
			},
			want:        nameOfFunc(increment),
			errContains: "",
		},
		{
			name:    "removeFile to removeFolder on success and case1",
			enabled: true,
			config:  removeFilesFoldersYml,
			failMsg: "removeFile should return removeFolder on success if dir is empty and currBlock is the last block",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(removeFilesFoldersYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				removeFile(i)
				increment(i)
				i.s.currBlock = i.s.subBlock + i.s.batchSize - 1
				return nameOfFunc(removeFile(i)), err
			},
			want:        nameOfFunc(removeFolder),
			errContains: "",
		},
		{
			name:    "removeFile to increment on success and case2",
			enabled: true,
			config:  removeFilesFoldersYml,
			failMsg: "removeFile should return removeFolder on success if dir is not empty",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(removeFilesFoldersYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				i.s.currBlock = i.s.subBlock + i.s.batchSize - 1
				return nameOfFunc(removeFile(i)), err
			},
			want:        nameOfFunc(increment),
			errContains: "",
		},
		{
			name:    "removeFolder to increment on success",
			enabled: true,
			config:  removeFilesFoldersYml,
			failMsg: "removeFolder should return increment on success",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(removeFilesFoldersYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				removeFile(i)
				increment(i)
				removeFile(i)
				return nameOfFunc(removeFolder(i)), err
			},
			want:        nameOfFunc(increment),
			errContains: "",
		},
		{
			name:    "removeFolder to increment if retries exceeds RetryCountOnFailure",
			enabled: true,
			config:  removeFilesFoldersYml,
			failMsg: "removeFolder should return increment after max retry",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(removeFilesFoldersYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				i.s.subBlockPath = "invalid"
				r := removeFolder
				for n := uint8(0); n < i.c.RetryCountOnFailure+1; n++ {
					r = r(i)
				}
				return nameOfFunc(r), err
			},
			want:        nameOfFunc(increment),
			errContains: "",
		},
		{
			name:    "increment to read on success",
			enabled: true,
			config:  removeFilesFoldersYml,
			failMsg: "increment should return read if toBlock is not specified",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(removeFilesFoldersYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				return nameOfFunc(increment(i)), err
			},
			want:        nameOfFunc(read),
			errContains: "",
		},
		{
			name:    "increment stops if toBlock is reached",
			enabled: true,
			config:  withToAndFromBlockYml,
			failMsg: "increment should return read if toBlock is and reached",
			args:    []interface{}{&sh},
			setup: func(args ...interface{}) {
				initConfig(withToAndFromBlockYml)
				createFiles(blocks, 0)
				args[0] = initStoreHandler()
			},
			teardown: func(args ...interface{}) {
				args[0].(*db.StoreHandler).Close()
				deleteFiles()
			},
			call: func(args ...interface{}) (interface{}, error) {
				i, err := New(args[0].(*db.StoreHandler))
				increment(i)
				return nameOfFunc(increment(i)), err
			},
			want:        nameOfFunc(stop),
			errContains: "",
		},
	}

	runTests(tests, t)

}

func runTests(tests []TestItem, t *testing.T) {
	for _, testItem := range tests {
		t.Run(testItem.name, func(t *testing.T) {
			if testItem.enabled {
				testItem.setup(testItem.args...)
				defer testItem.teardown(testItem.args...)

				got, err := testItem.call(testItem.args...)

				if testItem.want == nil {
					assert.Nil(t, got)
				} else {
					assert.Equal(t, testItem.want, got, testItem.failMsg)
				}

				if testItem.errContains != "" {
					assert.ErrorContains(t, err, testItem.errContains)
				} else {
					assert.Nil(t, err)
				}
			}
		})
	}
}

func initConfig(config string) {
	viper.SetConfigType("yml")
	err := viper.ReadConfig(strings.NewReader(config))
	if err != nil {
		panic(err)
	}
}

func initStoreHandler() *db.StoreHandler {
	bh, err1 := badger.NewBlockHandler()
	fh, err2 := badger.NewFilterHandler()
	if err1 != nil || err2 != nil {
		panic(fmt.Errorf("test initialization failed, [%s], [%s]", err1.Error(), err2.Error()))
	}
	return &db.StoreHandler{
		BlockHandler:  bh,
		FilterHandler: fh,
	}
}

func insertBlocks(sh *db.StoreHandler) {
	blocks := [...]*utils.Block{
		{Sequence: utils.IntToUint256(GenesisBlock), Hash: utils.HexStringToHash("a"), Transactions: []*utils.Transaction{{}}},
		{Sequence: utils.IntToUint256(GenesisBlock + 1), Hash: utils.HexStringToHash("b"), Transactions: []*utils.Transaction{{}, {}}},
		{Sequence: utils.IntToUint256(GenesisBlock + 2), Hash: utils.HexStringToHash("c"), Transactions: []*utils.Transaction{{}, {}, {}}},
		{Sequence: utils.IntToUint256(GenesisBlock + 3), Hash: utils.HexStringToHash("d"), Transactions: []*utils.Transaction{{}, {}, {}, {}}},
	}
	for _, b := range blocks {
		err := sh.InsertBlock(b)
		if err != nil {
			panic(err)
		}
	}
}

func createFiles(blocks []string, truncate int) {

	var block *utils.Block
	var buff []byte
	var err error
	var file *os.File

	for _, b := range blocks {
		buff = []byte(b)
		err = json.Unmarshal(buff, &block)
		if err != nil {
			panic(err)
		}

		bSize := uint64(viper.GetInt64("indexer.subFolderBatchSize"))
		bNum := block.Height
		bDir := bNum / bSize * bSize

		subDirName := filepath.Join(testDir, fmt.Sprintf("%d", bDir))
		err = os.MkdirAll(subDirName, 0755)
		if err != nil {
			panic(err)
		}
		fileName := filepath.Join(testDir, fmt.Sprintf("%d", bDir), fmt.Sprintf("%d.json", bNum))
		file, err = os.Create(fileName)
		if err != nil {
			panic(err)
		}

		if truncate > 0 {
			buff = buff[0:truncate]
		}

		_, err = file.Write(buff)

		if err != nil {
			panic(err)
		}
	}
}

func deleteFiles() {
	err := os.RemoveAll(testDir)
	if err != nil {
		panic(err)
	}
}

func nameOfFunc(i any) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
