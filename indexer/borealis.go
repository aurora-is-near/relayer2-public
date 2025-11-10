package indexer

import (
	"bytes"
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

const V1MessageVersion = 1

var cborDecMode cbor.DecMode

type UniqueID = [16]byte

type Envelope struct {
	_            struct{} `json:"-" cbor:",toarray"`
	Type         uint16
	SequentialID uint64
	TimestampS   uint32
	TimestampMS  uint16
	UniqueID     UniqueID
}

func getCborDecMode() (cbor.DecMode, error) {
	if cborDecMode != nil {
		return cborDecMode, nil
	}

	var err error
	cborDecMode, err = cbor.DecOptions{
		MaxArrayElements: 2147483647,
	}.DecMode()
	if err != nil {
		return nil, err
	}

	return cborDecMode, nil
}

func DecodeBorealisPayload[T any](data []byte) (*T, error) {
	reader := bytes.NewReader(data)

	var err error
	var version byte
	if version, err = reader.ReadByte(); err != nil {
		return nil, err
	}

	switch version {
	case V1MessageVersion:
		decMode, err := getCborDecMode()
		if err != nil {
			return nil, err
		}
		decoder := decMode.NewDecoder(reader)
		envelope := &Envelope{}
		if err := decoder.Decode(envelope); err != nil {
			return nil, err
		}
		payload := new(T)
		if err := decoder.Decode(payload); err != nil {
			return nil, err
		}
		return payload, nil
	default:
		return nil, fmt.Errorf("unknown version of borealis-message: %v", version)
	}
}
