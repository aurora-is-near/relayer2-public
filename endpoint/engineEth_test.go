package endpoint

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/aurora-is-near/relayer2-base/types/common"
	"github.com/aurora-is-near/relayer2-base/types/engine"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aurora-is-near/relayer2-base/db"
	"github.com/aurora-is-near/relayer2-base/db/badger"
	commonEndpoint "github.com/aurora-is-near/relayer2-base/endpoint"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

const (
	baseUrl                       = "https://testnet.aurora.dev:443"
	zeroAddress                   = "0x0000000000000000000000000000000000000000"
	fromAddress                   = "0xC4CD073a8868dd94df8a10DA4e3639CE9534536F"
	toAddress                     = "0x9d9bD1909550Cb8BEEd9ce7B291DC4DaCD85d39d"
	transferValue                 = 1000000000                                                                   // 1 Giga wei transfer value
	transferValueOOF              = 1000000000000000000                                                          // 1 eth Out Of Fund transfer value
	contractAddress               = "0x022F4C17628A68E0a5C39D8155216ba95Fa9e08a"                                 // testnet CA that returns "0x5" value
	contractAddressStackOverFlow  = "0xB42A5f475216E4508ba22dFe21a0669c40BE2936"                                 // testnet CA that returns StackOverflow error
	contractAddressCallTooDeep    = "0x73655f6d514b05ef1BEc27CfaCA01af5DA6B4388"                                 // testnet CA that returns Unreachable error
	contractAddressOutOfOffset    = "0x4d0FcE38D795155b1A72e55423d3696d4aDff53b"                                 // testnet CA that returns Unreachable error Out Of Offset error
	contractCheckMethodData       = "0x919840ad"                                                                 // 4 bytes of check methods keccak256 signature -> "check()"
	contractCallToDeepMethodData  = "0x395942a4"                                                                 // 4 bytes of callToDeep methods keccak256 signature -> "callToDeep()"
	contractCallTestMethodData    = "0xf8a8fd6d"                                                                 // 4 bytes of test methods keccak256 signature -> "test()"
	contractCallTestOOOMethodData = "0xbb29998e000000000000000000000000f53cC0ef22a093436bb53478b6b3fa8922264a70" // 4 bytes of test methods keccak256 signature -> "test(address)" + address 0xF53cC0eF22a093436BB53478b6B3FA8922264a70 (complete it to 32 bytes)

	txsInvalidNonceStr   = "0xf86d8202fe80825208949d9bd1909550cb8beed9ce7b291dc4dacd85d39d888ac7230489e8000080849c8a82c9a0f17bc4738b358e045850ede1ee86afd8b4cdc58789eda6bf5fe12d2b364e0816a00787a98813e68a58ddcef2cd0b2dd5b90785c32726baa27d3b13e9da299a28cf"
	txsInvalidRawDataStr = "0xf86d8202fe80825208949d9bd1909550cb8beed9ce7b291dc4dacd85d39d888ac7230489e8000080849c8a82c9a0f17bc4738b358e045850ede1ee86afd8b4cdc58789eda6bf5fe12d2b364e0816a00787a98813e68a58ddcef2cd0b2dd5b90785c32726baa27d3b13e9da299"
	txsLowGasPriceStr    = "0xf86a820315808259d8949d9bd1909550cb8beed9ce7b291dc4dacd85d39d85e8d4a5100080849c8a82caa09b3661dbc1cdc757d0f566c96e08718033196ef6293962b0a28199494331ba9fa0287398e2db4c3ff130f7beafbcd1ad763df70549724b14db8686fbcf2639b699"
	txsLowGasLimitStr    = "0xf8688203158064949d9bd1909550cb8beed9ce7b291dc4dacd85d39d85e8d4a5100080849c8a82caa02e3db9cf6ca6ef9f164ff537f1894e2fe981c4b65e29bf899da2a08f1fac5df4a023957b8b64c799da9161f47bee9d1b72cb55e5663b54b1b6190f3485c652dd76"
	timeoutSec           = 60
)

const engineEthTestFailYaml1 = `
endpoint:
  engine:
`
const engineEthTestFailYaml2 = `
endpoint:
  engine:
    nearNetworkID: testnet
    nearNodeURL: https://rpc.testnet.near.org
    signer: asd.testnet
`

const engineEthTestYaml = `
endpoint:
  engine:
    nearNetworkID: testnet
    nearNodeURL: https://rpc.testnet.near.org
    signer: tolgcoplu.testnet
    SignerKey: ../config/tolgcoplu.testnet.json
    minGasPrice: 0
    minGasLimit: 21000
    gasForNearTxsCall: 300000000000000
    depositForNearTxsCall: 0
    retryWaitTimeMsForNearTxsCall: 3000
    retryNumberForNearTxsCall: 3
`

var handler db.Handler

var rpcClientBase *rpc.Client
var baseEndpoint *commonEndpoint.Endpoint
var engineEth *EngineEth
var engineNet *EngineNet
var fromAddr common.Address
var toAddr common.Address
var transferVal common.Uint256
var transferValOOF common.Uint256
var contractAddr common.Address
var contractAddrStackOverFlow common.Address
var contractAddrCallTooDeep common.Address
var contractAddrOutOfOffset common.Address
var contractData common.DataVec
var contractDataStackOverFlow common.DataVec
var contractDataCallTooDeep common.DataVec
var contractDataOutOfOffset common.DataVec
var txsInvalidNonce common.DataVec
var txsInvalidRawData common.DataVec
var txsLowGasPrice common.DataVec
var txsLowGasLimit common.DataVec

func initializeStoreHandler() db.Handler {
	bh, err := badger.NewBlockHandler()
	if err != nil {
		panic(err)
	}
	fh, err := badger.NewFilterHandler()
	if err != nil {
		panic(err)
	}
	return db.StoreHandler{
		BlockHandler:  bh,
		FilterHandler: fh,
	}
}

func getConfig() {
	viper.SetConfigType("yml")
	err := viper.ReadConfig(strings.NewReader(engineEthTestYaml))
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	// Create an object from EngineEth struct
	handler = initializeStoreHandler()
	defer handler.Close()
	getConfig()

	baseEndpoint = commonEndpoint.New(handler)
	engineEth = NewEngineEth(baseEndpoint)
	engineNet = NewEngineNet(engineEth)

	// Create a rpc client to run JSON RPC calls and compare the results
	rpcClientBase, _ = rpc.DialContext(context.Background(), baseUrl)

	// Create from, to, and contract addresses to use in the tests
	fromAddr = common.HexStringToAddress(fromAddress)
	toAddr = common.HexStringToAddress(toAddress)
	transferVal = common.IntToUint256(transferValue)
	transferValOOF = common.IntToUint256(transferValueOOF)
	contractAddr = common.HexStringToAddress(contractAddress)
	contractAddrStackOverFlow = common.HexStringToAddress(contractAddressStackOverFlow)
	contractAddrCallTooDeep = common.HexStringToAddress(contractAddressCallTooDeep)
	contractAddrOutOfOffset = common.HexStringToAddress(contractAddressOutOfOffset)

	contractData, _ = hex.DecodeString(contractCheckMethodData[2:])
	contractDataStackOverFlow, _ = hex.DecodeString(contractCallToDeepMethodData[2:])
	contractDataCallTooDeep, _ = hex.DecodeString(contractCallTestMethodData[2:])
	contractDataOutOfOffset, _ = hex.DecodeString(contractCallTestOOOMethodData[2:])
	txsInvalidNonce, _ = hex.DecodeString(txsInvalidNonceStr[2:])
	txsInvalidRawData, _ = hex.DecodeString(txsInvalidRawDataStr[2:])
	txsLowGasLimit, _ = hex.DecodeString(txsLowGasLimitStr[2:])
	txsLowGasPrice, _ = hex.DecodeString(txsLowGasPriceStr[2:])

	// If no default provided user the random generated addresses
	if fromAddr.String() == zeroAddress {
		rand.Read(fromAddr.Address[:])
	}
	if toAddr.String() == zeroAddress {
		rand.Read(toAddr.Address[:])
	}
	if contractAddr.String() == zeroAddress {
		rand.Read(contractAddr.Address[:])
	}

	exitVal := m.Run()
	os.Exit(exitVal)
}

func TestNewEngineEthErrors(t *testing.T) {
	data := []struct {
		name       string
		testConfig string
	}{
		{"empty_config", engineEthTestFailYaml1},
		{"false_config", engineEthTestFailYaml2},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			err := viper.ReadConfig(strings.NewReader(d.testConfig))
			if err != nil {
				panic(err)
			}
			baseEndpoint := commonEndpoint.New(handler)
			// Check if default engineEth config parameters set OK
			assert.Panics(t, func() { NewEngineEth(baseEndpoint) }, "expected panic, should check the engineEth config")
		})

	}
}

func TestNetEndpointsCompare(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutSec*time.Second)
	defer cancel()

	resp, err := engineNet.Version(ctx)
	if err != nil {
		t.Error("Version request error:", err)
	}
	respHex := *resp

	var respExpected string
	err = rpcClientBase.CallContext(ctx, &respExpected, "net_version")
	if err != nil {
		t.Log("request error:", err)
	}

	if (respHex == "" && respExpected == "") || respExpected != respHex {
		t.Errorf("incorrect result: expected %s, got %s", respExpected, respHex)
	}
}

func TestEthEndpointsCompare(t *testing.T) {
	SafeBlockNumber := common.IntToBN64(-4)
	FinalizedBlockNumber := common.IntToBN64(-3)
	PendingBlockNumber := common.IntToBN64(-2)
	LatestBlockNumber := common.IntToBN64(-1)

	ctx, cancel := context.WithTimeout(context.Background(), timeoutSec*time.Second)
	defer cancel()

	data := []struct {
		name   string
		api    string
		method string
		args   []interface{}
	}{
		{"test eth_chainId", "eth_chainId", "ChainId", []interface{}{ctx}},
		{"test eth_getCode with EOA and safe block num", "eth_getCode", "GetCode", []interface{}{ctx, toAddr, &SafeBlockNumber}},
		{"test eth_getCode with safe block num", "eth_getCode", "GetCode", []interface{}{ctx, contractAddr, &SafeBlockNumber}},
		{"test eth_getCode with finalized block num", "eth_getCode", "GetCode", []interface{}{ctx, contractAddr, &FinalizedBlockNumber}},
		{"test eth_getCode with pending block num", "eth_getCode", "GetCode", []interface{}{ctx, contractAddr, &PendingBlockNumber}},
		{"test eth_getCode with latest block num", "eth_getCode", "GetCode", []interface{}{ctx, contractAddr, &LatestBlockNumber}},
		{"test eth_getBalance with latest block num", "eth_getBalance", "GetBalance", []interface{}{ctx, fromAddr, &LatestBlockNumber}},
		{"test eth_getTransactionCount with latest block num", "eth_getTransactionCount", "GetTransactionCount", []interface{}{ctx, fromAddr, &LatestBlockNumber}},
		{"test eth_getStorageAt with latest block num", "eth_getStorageAt", "GetStorageAt", []interface{}{ctx, contractAddr, common.IntToUint256(0), &LatestBlockNumber}},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			// Use an rpc client to get the expected value
			var expectedStr string
			var tmpArgs []interface{}
			if len(d.args) > 2 {
				tmpArgs = d.args[1 : len(d.args)-1]
			} else if len(d.args) == 2 {
				tmpArgs = d.args[1:]
			}

			err := rpcClientBase.CallContext(ctx, &expectedStr, d.api, tmpArgs...)
			if err != nil {
				t.Log(d.api, " client base error:", err)
			}

			// Call the target api
			resp, err := Invoke(ctx, engineEth, d.method, d.args...)
			if err != nil {
				t.Error(d.api, " request error:", err)
			}
			// Compare the retrieved response and expected result
			switch v := resp.(type) {
			case *common.Uint256:
				// leading zeros should be checked and removed (if any)
				tmpStr := ""
				for expectedStr != tmpStr {
					tmpStr = expectedStr
					expectedStr = "0x" + strings.TrimPrefix(expectedStr[2:len(expectedStr)-1], "0") + expectedStr[len(expectedStr)-1:]
				}
				expectedUint256, err := common.Uint256FromHex(expectedStr)
				if err != nil {
					t.Errorf("error while decoding string to Uint256: %v", err)
				}
				if v.Cmp(*expectedUint256) != 0 {
					t.Errorf("incorrect response: expected 0x%s, got 0x%s", expectedUint256.Text(16), v.Text(16))
				}
			case *string:
				if *v != expectedStr {
					t.Errorf("incorrect response: expected %s, got %s", expectedStr, *v)
				}
			default:
				t.Errorf("incorrect type in response")
			}
		})
	}
}

func TestEthEndpointsStatic(t *testing.T) {
	LatestBlockNumber := common.IntToBN64(-1)

	ctx, cancel := context.WithTimeout(context.Background(), timeoutSec*time.Second)
	defer cancel()

	data := []struct {
		name           string
		api            string
		method         string
		args           []interface{}
		isAsyncCall    bool
		expectedResult string
	}{
		{"test aysnc eth_sendRawTransaction incorrect nonce", "eth_sendRawTransaction", "SendRawTransaction", []interface{}{ctx, txsInvalidNonce}, true, "anyHash"},
		{"test sync eth_sendRawTransaction incorrect txs raw data", "eth_sendRawTransaction", "SendRawTransaction", []interface{}{ctx, txsInvalidRawData}, false, "transaction parameter is not correct"},
		// Needs changes on engine side to be able to run this test. Therefore, it is commented out for now
		// {"test sync eth_sendRawTransaction low gas price", "eth_sendRawTransaction", "SendRawTransaction", []interface{}{ctx, txsLowGasPrice}, false, "gas price too low"},
		{"test sync eth_sendRawTransaction low gas limit", "eth_sendRawTransaction", "SendRawTransaction", []interface{}{ctx, txsLowGasLimit}, false, "intrinsic gas too low"},
		{"test sync eth_sendRawTransaction incorrect nonce", "eth_sendRawTransaction", "SendRawTransaction", []interface{}{ctx, txsInvalidNonce}, false, "ERR_INCORRECT_NONCE"},
		{"test eth_call contract data", "eth_call", "Call", []interface{}{ctx, engine.TransactionForCall{From: &fromAddr, To: &contractAddr, Data: contractData}, &LatestBlockNumber}, false, "0x0000000000000000000000000000000000000000000000000000000000000005"},
		{"test eth_call transfer to EOA", "eth_call", "Call", []interface{}{ctx, engine.TransactionForCall{From: &fromAddr, To: &toAddr, Value: &transferVal}, &LatestBlockNumber}, false, "0x"},
		// Needs changes on engine side to be able to run this test properly. Normally, "execution error: Out Of Gas" should be retrieved. Hovewer, since max gas is staticilly applied the result seems to be success
		{"test eth_call out of gas", "eth_call", "Call", []interface{}{ctx, engine.TransactionForCall{From: &fromAddr, To: &toAddr, Value: &transferVal, Gas: &transferValOOF}, &LatestBlockNumber}, false, "0x"},
		{"test eth_call out of fund", "eth_call", "Call", []interface{}{ctx, engine.TransactionForCall{From: &fromAddr, To: &toAddr, Value: &transferValOOF}, &LatestBlockNumber}, false, "Ok(OutOfFund)"},
		{"test eth_call stack overflow", "eth_call", "Call", []interface{}{ctx, engine.TransactionForCall{To: &contractAddrStackOverFlow, Data: contractDataStackOverFlow}, &LatestBlockNumber}, false, "EvmError(StackOverflow)"},
		// Testnet returns "0x" for Revert status, following the same approach
		{"test eth_call call too deep", "eth_call", "Call", []interface{}{ctx, engine.TransactionForCall{To: &contractAddrCallTooDeep, Data: contractDataCallTooDeep}, &LatestBlockNumber}, false, "execution reverted without data"},
		{"test eth_call out of offset", "eth_call", "Call", []interface{}{ctx, engine.TransactionForCall{From: &fromAddr, To: &contractAddrOutOfOffset, Data: contractDataOutOfOffset}, &LatestBlockNumber}, false, "Ok(OutOfOffset)"},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			engineEth.Config.EngineConfig.AsyncSendRawTxs = d.isAsyncCall
			// Call the target api
			resp, err := Invoke(ctx, engineEth, d.method, d.args...)

			// Compare the retrieved response/error and expected result
			switch v := resp.(type) {
			case *string:
				if d.expectedResult == "anyHash" {
					tmp := *v
					if len(tmp) != 66 {
						t.Errorf("incorrect response: expected %s, got %s", d.expectedResult, *v)
					}
				} else if *v != d.expectedResult {
					t.Errorf("incorrect response: expected %s, got %s", d.expectedResult, *v)
				}
			default:
				if err.Error() != d.expectedResult {
					t.Errorf("incorrect response: expected %s, got %s", d.expectedResult, err.Error())
				}
			}
		})
	}
}

func Invoke(ctx context.Context, obj interface{}, method string, args ...interface{}) (interface{}, error) {
	inputs := make([]reflect.Value, len(args))
	for i := range args {
		inputs[i] = reflect.ValueOf(args[i])
	}
	values := reflect.ValueOf(obj).MethodByName(method).Call(inputs)

	err := values[1].Interface()
	if err != nil {
		return nil, err.(error)
	}
	return values[0].Interface(), nil
}
