package rollupsyncservice

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/core"
	"github.com/scroll-tech/go-ethereum/core/rawdb"
	"github.com/scroll-tech/go-ethereum/ethdb/memorydb"
	"github.com/scroll-tech/go-ethereum/node"
	"github.com/scroll-tech/go-ethereum/params"
)

func TestRollupSyncServiceStartAndStop(t *testing.T) {
	genesisConfig := &params.ChainConfig{
		Scroll: params.ScrollConfig{
			L1Config: &params.L1Config{
				L1ChainId:          11155111,
				ScrollChainAddress: common.HexToAddress("0x2D567EcE699Eabe5afCd141eDB7A4f2D0D6ce8a0"),
			},
		},
	}
	db := rawdb.NewDatabase(memorydb.New())
	l1Client := &mockEthClient{}
	bc := &core.BlockChain{}
	node := &node.Node{}
	service, err := NewRollupSyncService(context.Background(), genesisConfig, db, l1Client, bc, node, 1)
	if err != nil {
		t.Fatalf("Failed to new rollup sync service: %v", err)
	}

	assert.NotNil(t, service)
	service.Start()
	time.Sleep(10 * time.Millisecond)
	service.Stop()
}

func TestDecodeChunkRanges(t *testing.T) {
	scrollChainABI, err := scrollChainMetaData.GetAbi()
	require.NoError(t, err)

	service := &RollupSyncService{
		scrollChainABI: scrollChainABI,
	}

	data, err := os.ReadFile("testdata/commit_batch_transaction.json")
	require.NoError(t, err, "Failed to read json file")

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
