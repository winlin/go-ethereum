package rollupsyncservice

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/scroll-tech/go-ethereum"
	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/core/types"
)

func TestL1Client(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockEthClient{}

	scrollChainABI, err := scrollChainMetaData.GetAbi()
	if err != nil {
		t.Fatal("failed to get scroll chain abi", "err", err)
	}
	scrollChainAddress := common.HexToAddress("0x0123456789abcdef")
	l1Client, err := newL1Client(ctx, mockClient, 11155111, scrollChainAddress, scrollChainABI)
	assert.Nil(t, err, "Failed to initialize L1Client")

	blockNumber, err := l1Client.getLatestFinalizedBlockNumber(ctx)
	assert.Nil(t, err, "Error getting latest confirmed block number")
	assert.Equal(t, uint64(36), blockNumber, "Unexpected block number")

	logs, err := l1Client.fetchRollupEventsInRange(ctx, 0, blockNumber)
	assert.Empty(t, logs, "Expected no logs from fetchRollupEventsInRange")
	assert.Nil(t, err, "Error fetching rollup events in range")
}

type mockEthClient struct{}

func (m *mockEthClient) BlockNumber(ctx context.Context) (uint64, error) {
	return 11155111, nil
}

func (m *mockEthClient) ChainID(ctx context.Context) (*big.Int, error) {
	return big.NewInt(11155111), nil
}

func (m *mockEthClient) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	return []types.Log{}, nil
}

func (m *mockEthClient) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	return &types.Header{
		Number: big.NewInt(100 - 64),
	}, nil
}

func (m *mockEthClient) SubscribeFilterLogs(ctx context.Context, query ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error) {
	return nil, nil
}

func (m *mockEthClient) TransactionByHash(ctx context.Context, txHash common.Hash) (tx *types.Transaction, isPending bool, err error) {
	return &types.Transaction{}, false, nil
}
