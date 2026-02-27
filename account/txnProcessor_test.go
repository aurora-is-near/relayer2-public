package account

import (
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	"github.com/aurora-is-near/relayer2-base/endpoint"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testChainID is the Aurora testnet chain ID used for test transaction signing.
var testChainID = big.NewInt(1313161555)

// newTestTxnProcessor creates a minimal TxnProcessor suitable for unit testing NewTxnReq.
// It does not require a NEAR connection.
func newTestTxnProcessor(minGasPrice uint64, minGasLimit uint64) *TxnProcessor {
	return &TxnProcessor{
		mapper: CRC32Mapper{},
		config: &endpoint.EngineConfig{
			MinGasPrice: new(big.Int).SetUint64(minGasPrice),
			MinGasLimit: minGasLimit,
		},
		keys: []string{"ed25519:testkey"},
	}
}

// testKey generates an ECDSA private key for test signing.
func testKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	return key
}

// signAndEncode signs a transaction and returns its RLP-encoded bytes.
func signAndEncode(t *testing.T, tx *types.Transaction, signer types.Signer, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	signed, err := types.SignTx(tx, signer, key)
	require.NoError(t, err)
	raw, err := signed.MarshalBinary()
	require.NoError(t, err)
	return raw
}

// uint256FromBig converts a *big.Int to a *uint256.Int for use in go-ethereum EIP-7702 tx types.
func uint256FromBig(b *big.Int) *uint256.Int {
	u, _ := uint256.FromBig(b)
	return u
}

func TestNewTxnReq_LegacyType0(t *testing.T) {
	tp := newTestTxnProcessor(0, 21000)
	key := testKey(t)
	signer := types.LatestSignerForChainID(testChainID)

	tx := types.NewTx(&types.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1000000000),
		Gas:      21000,
		To:       &common.Address{},
		Value:    big.NewInt(0),
	})
	raw := signAndEncode(t, tx, signer, key)

	req, err := tp.NewTxnReq(raw)
	require.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, uint64(0), req.nonce)
}

func TestNewTxnReq_Type2_EIP1559(t *testing.T) {
	tp := newTestTxnProcessor(0, 21000)
	key := testKey(t)
	signer := types.LatestSignerForChainID(testChainID)

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   testChainID,
		Nonce:     5,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(2000000000),
		Gas:       21000,
		To:        &common.Address{},
		Value:     big.NewInt(0),
	})
	raw := signAndEncode(t, tx, signer, key)

	req, err := tp.NewTxnReq(raw)
	require.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, uint64(5), req.nonce)
}

func TestNewTxnReq_Type4_EIP7702(t *testing.T) {
	tp := newTestTxnProcessor(0, 21000)
	key := testKey(t)
	signer := types.LatestSignerForChainID(testChainID)

	authKey := testKey(t)
	authAddr := crypto.PubkeyToAddress(authKey.PublicKey)

	auth := types.SetCodeAuthorization{
		ChainID: *uint256FromBig(testChainID),
		Address: authAddr,
		Nonce:   0,
	}
	signedAuth, err := types.SignSetCode(authKey, auth)
	require.NoError(t, err)

	to := common.HexToAddress("0x9d9bD1909550Cb8BEEd9ce7B291DC4DaCD85d39d")
	tx := types.NewTx(&types.SetCodeTx{
		ChainID:   uint256FromBig(testChainID),
		Nonce:     20,
		GasTipCap: uint256FromBig(big.NewInt(1000000000)),
		GasFeeCap: uint256FromBig(big.NewInt(2000000000)),
		Gas:       50000,
		To:        to,
		Value:     uint256FromBig(big.NewInt(0)),
		AuthList:  []types.SetCodeAuthorization{signedAuth},
	})
	raw := signAndEncode(t, tx, signer, key)

	req, err := tp.NewTxnReq(raw)
	require.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, uint64(20), req.nonce)

	// Verify sender can be extracted (compare lowercase since relayer2-base uses lowercase hex)
	expectedAddr := crypto.PubkeyToAddress(key.PublicKey)
	assert.Equal(t, strings.ToLower(expectedAddr.Hex()), req.sender.Hex())
}

func TestNewTxnReq_Type4_GasLimitValidation(t *testing.T) {
	// Set minGasLimit to 100000, but transaction has 21000 -> should fail
	tp := newTestTxnProcessor(0, 100000)
	key := testKey(t)
	signer := types.LatestSignerForChainID(testChainID)

	authKey := testKey(t)
	authAddr := crypto.PubkeyToAddress(authKey.PublicKey)

	auth := types.SetCodeAuthorization{
		ChainID: *uint256FromBig(testChainID),
		Address: authAddr,
		Nonce:   0,
	}
	signedAuth, err := types.SignSetCode(authKey, auth)
	require.NoError(t, err)

	to := common.HexToAddress("0x9d9bD1909550Cb8BEEd9ce7B291DC4DaCD85d39d")
	tx := types.NewTx(&types.SetCodeTx{
		ChainID:   uint256FromBig(testChainID),
		Nonce:     0,
		GasTipCap: uint256FromBig(big.NewInt(1000000000)),
		GasFeeCap: uint256FromBig(big.NewInt(2000000000)),
		Gas:       21000,
		To:        to,
		Value:     uint256FromBig(big.NewInt(0)),
		AuthList:  []types.SetCodeAuthorization{signedAuth},
	})
	raw := signAndEncode(t, tx, signer, key)

	_, err = tp.NewTxnReq(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intrinsic gas too low")
}

func TestNewTxnReq_GasPriceTooLow(t *testing.T) {
	// minGasPrice=1 gwei, transaction has gasPrice=0 -> should fail
	tp := newTestTxnProcessor(1000000000, 21000)
	key := testKey(t)
	signer := types.LatestSignerForChainID(testChainID)

	tx := types.NewTx(&types.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(0),
		Gas:      21000,
		To:       &common.Address{},
		Value:    big.NewInt(0),
	})
	raw := signAndEncode(t, tx, signer, key)

	_, err := tp.NewTxnReq(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gas price too low")
}

func TestNewTxnReq_InvalidRawData(t *testing.T) {
	tp := newTestTxnProcessor(0, 21000)

	_, err := tp.NewTxnReq([]byte{0xde, 0xad, 0xbe, 0xef})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode transaction")
}
