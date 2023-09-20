package rollupsyncservice

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scroll-tech/go-ethereum/core/rawdb"
)

func TestDecodeChunkRanges(t *testing.T) {
	scrollChainABI, err := scrollChainMetaData.GetAbi()
	require.NoError(t, err)

	service := &RollupSyncService{
		scrollChainABI: scrollChainABI,
	}

	data, err := os.ReadFile("testdata/commit_batch_transaction.json")
	require.NoError(t, err, "Failed to read json file")

	// Modify the structure to match the new JSON format
	type transactionJson struct {
		CallData string `json:"calldata"`
	}
	var txObj transactionJson
	err = json.Unmarshal(data, &txObj)
	require.NoError(t, err, "Failed to unmarshal transaction json")

	testTxData, err := hex.DecodeString(txObj.CallData[2:])
	if err != nil {
		t.Fatalf("Failed to decode string: %v", err)
	}

	ranges, err := service.decodeChunkRanges(testTxData)
	if err != nil {
		t.Fatalf("Failed to decode chunk ranges: %v", err)
	}

	expectedRanges := []*rawdb.ChunkBlockRange{
		{StartBlockNumber: 335921, EndBlockNumber: 335928},
		{StartBlockNumber: 335929, EndBlockNumber: 335933},
		{StartBlockNumber: 335934, EndBlockNumber: 335938},
		{StartBlockNumber: 335939, EndBlockNumber: 335942},
		{StartBlockNumber: 335943, EndBlockNumber: 335945},
		{StartBlockNumber: 335946, EndBlockNumber: 335949},
		{StartBlockNumber: 335950, EndBlockNumber: 335956},
		{StartBlockNumber: 335957, EndBlockNumber: 335962},
	}

	if len(expectedRanges) != len(ranges) {
		t.Fatalf("Expected range length %v, got %v", len(expectedRanges), len(ranges))
	}

	for i := range ranges {
		if *expectedRanges[i] != *ranges[i] {
			t.Fatalf("Mismatch at index %d: expected %v, got %v", i, *expectedRanges[i], *ranges[i])
		}
	}
}
