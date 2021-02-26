package stateproof

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/hex"
	"github.com/hashgraph/hedera-state-proof-verifier-go/internal/errors"
	"github.com/hashgraph/hedera-state-proof-verifier-go/internal/parser"
	"github.com/hashgraph/hedera-state-proof-verifier-go/internal/types"
	"math"
	"regexp"
)

func Verify(txId string, payload []byte) (bool, error) {
	regex := regexp.MustCompile("[.@\\-]")
	txId = regex.ReplaceAllString(txId, "_")

	stateProof, err := types.NewStateProof(payload)
	if err != nil {
		return false, err
	}

	nodeIdPairs, err := parser.ParseAddressBooks(stateProof.AddressBooks)
	if err != nil {
		return false, err
	}

	signatureFiles, err := parser.ParseSignatureFiles(stateProof.SignatureFiles)
	if err != nil {
		return false, err
	}

	recordFile, err := parser.ParseRecordFile(stateProof.RecordFile)
	if err != nil {
		return false, err
	}

	if recordFile.TransactionsMap[txId] == nil {
		return false, errors.ErrorTransactionNotFound
	}

	err = performStateProof(nodeIdPairs, signatureFiles, recordFile.Hash)
	if err != nil {
		return false, err
	}

	return true, nil
}

func performStateProof(nodeIdPubKeyPairs map[string]string, signatureFiles map[string]*types.SignatureFile, hash string) error {
	res, err := verifySignatures(nodeIdPubKeyPairs, signatureFiles)
	if err != nil {
		return err
	}

	if hash != res {
		return errors.ErrorHashesNotMatch
	}
	return nil
}

func verifySignatures(nodeIdPubKeyPairs map[string]string, signatureFiles map[string]*types.SignatureFile) (string, error) {
	verifiedSigs := make(map[string][]string)
	consensusHash := ""
	maxHashCount := 0

	for nodeId, sigFile := range signatureFiles {
		pubKey := nodeIdPubKeyPairs[nodeId]
		if !verifySignature(pubKey, sigFile.Hash, sigFile.Signature) {
			return "", errors.ErrorVerifySignature
		}

		if sigFile.MetadataHash != nil && !verifySignature(pubKey, sigFile.MetadataHash, sigFile.MetadataSignature) {
			return "", errors.ErrorVerifyMetadataSignature
		}

		hexHash := hex.EncodeToString(sigFile.Hash)
		verifiedSigs[hexHash] = append(verifiedSigs[hexHash], nodeId)

		nodesCount := len(verifiedSigs[hexHash])
		if nodesCount > 1 && nodesCount > maxHashCount {
			maxHashCount = nodesCount
			consensusHash = hexHash
		}
	}

	consensusSigs := int(math.Ceil(float64(len(signatureFiles) / 3)))
	if maxHashCount >= consensusSigs {
		return consensusHash, nil
	} else {
		return "", nil
	}
}

func verifySignature(publicKeyString string, hash []byte, sig []byte) bool {
	pkBytes, err := hex.DecodeString(publicKeyString)
	if err != nil {
		return false
	}
	pk, err := x509.ParsePKIXPublicKey(pkBytes)
	if err != nil {
		return false
	}

	hashedHash := sha512.Sum384(hash)

	res := rsa.VerifyPKCS1v15(pk.(*rsa.PublicKey), crypto.SHA384, hashedHash[:], sig)
	if res != nil {
		return false
	}

	return true
}
