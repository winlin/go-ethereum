package rollupsyncservice

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/scroll-tech/go-ethereum/crypto"
)

func TestEventSignatures(t *testing.T) {
	scrollChainABI, err := scrollChainMetaData.GetAbi()
	if err != nil {
		t.Fatal("failed to get scroll chain abi", "err", err)
	}

	assert.Equal(t, crypto.Keccak256Hash([]byte("CommitBatch(uint256,bytes32)")), scrollChainABI.Events["CommitBatch"].ID)
	assert.Equal(t, crypto.Keccak256Hash([]byte("RevertBatch(uint256,bytes32)")), scrollChainABI.Events["RevertBatch"].ID)
	assert.Equal(t, crypto.Keccak256Hash([]byte("FinalizeBatch(uint256,bytes32,bytes32,bytes32)")), scrollChainABI.Events["FinalizeBatch"].ID)
}
