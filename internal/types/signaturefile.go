package types

import (
	"bytes"
	"encoding/binary"
	"github.com/limechain/hedera-state-proof-verifier-go/internal/constants"
	"github.com/limechain/hedera-state-proof-verifier-go/internal/errors"
	"github.com/limechain/hedera-state-proof-verifier-go/internal/reader"
)

type SignatureFile struct {
	Stream
	Hash              []byte
	Signature         []byte
	Version           int
	MetadataHash      []byte
	MetadataSignature []byte
}

func NewSignatureFile(buffer *bytes.Reader) (*SignatureFile, error) {
	stream, err := NewStream(buffer)
	if err != nil {
		return nil, err
	}
	signatureFile := &SignatureFile{
		Stream: *stream,
	}

	bodyLength, err := signatureFile.readBody(buffer)
	if err != nil {
		return nil, err
	}

	signatureFile.BodyLength = *bodyLength

	return signatureFile, nil
}

func (sf *SignatureFile) readBody(buffer *bytes.Reader) (*uint32, error) {
	var sigFileType uint32
	err := binary.Read(buffer, binary.BigEndian, &sigFileType)
	if err != nil {
		return nil, err
	}

	if sigFileType != constants.Sha384WithRsaType {
		return nil, errors.ErrorInvalidSignatureFileType
	}

	length, b, err := reader.LengthAndBytes(buffer, constants.ByteSize, constants.Sha384WithRsaMaxLength, true)
	if err != nil {
		return nil, err
	}
	sf.Signature = b

	finalLength := *length + constants.IntSize

	return &finalLength, nil
}

func NewV2SignatureFile(buffer *bytes.Reader) (*SignatureFile, error) {
	hash := make([]byte, constants.Sha384Length)

	_, err := buffer.Read(hash)
	if err != nil {
		return nil, err
	}

	signatureMarker, err := buffer.ReadByte()
	if err != nil {
		return nil, err
	}

	if signatureMarker != constants.SignatureFileV2Marker {
		return nil, errors.ErrorUnexpectedSignatureFileTypeDelimiter
	}

	// signature length and actual signature
	_, signature, err := reader.LengthAndBytes(buffer, constants.ByteSize, constants.Sha384WithRsaMaxLength, false)
	if err != nil {
		return nil, err
	}

	if buffer.Len() != 0 {
		return nil, errors.ErrorExtraDataInSignatureFile
	}

	return &SignatureFile{
		Stream:    Stream{},
		Hash:      hash,
		Signature: signature,
		Version:   constants.SignatureFileFormatV4,
	}, nil
}

func NewV5SignatureFile(buffer *bytes.Reader) (*SignatureFile, error) {
	// object stream signature version
	err := binary.Read(buffer, binary.BigEndian, make([]byte, constants.IntSize))
	if err != nil {
		return nil, err
	}

	// hash of the entire corresponding stream file
	hash, err := NewHash(buffer)
	if err != nil {
		return nil, err
	}

	// signature, generated by signing the hash bytes
	signatureFile, err := NewSignatureFile(buffer)
	if err != nil {
		return nil, err
	}

	// metadata hash of the corresponding stream file
	metadataHash, err := NewHash(buffer)
	if err != nil {
		return nil, err
	}

	// todo:
	metadataSigFile, err := NewSignatureFile(buffer)
	if err != nil {
		return nil, err
	}

	if buffer.Len() != 0 {
		return nil, errors.ErrorExtraDataInSignatureFile
	}

	return &SignatureFile{
		Stream:            Stream{},
		Hash:              hash.Hash,
		Signature:         signatureFile.Signature,
		Version:           constants.SignatureFileFormatV5,
		MetadataHash:      metadataHash.Hash,
		MetadataSignature: metadataSigFile.Signature,
	}, nil
}
