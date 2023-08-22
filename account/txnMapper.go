package account

import (
	"hash/crc32"
	"sync/atomic"
)

type TxnMapper interface {
	Map(sender string, totalNumKeys int) (keyIndex int)
}

// CRC32Mapper is a signer to key mapper implementation that uses crc32 hash to consistently distributes signers to keys.
// This ensures that same signers are mapped to same key
type CRC32Mapper struct{}

func (m CRC32Mapper) Map(sender string, totalNumKeys int) int {
	idx := crc32.ChecksumIEEE([]byte(sender)) % uint32(totalNumKeys)
	return int(idx)
}

// RoundRobinMapper is a signer to key mapper implementation that equally distributes signers to keys
type RoundRobinMapper struct {
	offset uint32
}

func (m RoundRobinMapper) Map(_ string, totalNumKeys int) int {
	return m.rrMap(totalNumKeys)
}

func (m RoundRobinMapper) rrMap(totalNumKeys int) int {
	offset := atomic.AddUint32(&m.offset, 1) - 1
	idx := offset % uint32(totalNumKeys)
	return int(idx)
}
