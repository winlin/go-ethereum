package rollupsyncservice

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/scroll-tech/go-ethereum/accounts/abi"
	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/core"
	"github.com/scroll-tech/go-ethereum/core/rawdb"
	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/ethdb"
	"github.com/scroll-tech/go-ethereum/log"
	"github.com/scroll-tech/go-ethereum/node"
	"github.com/scroll-tech/go-ethereum/params"

	"github.com/scroll-tech/go-ethereum/rollup/rcfg"
	"github.com/scroll-tech/go-ethereum/rollup/sync_service"
	"github.com/scroll-tech/go-ethereum/rollup/withdrawtrie"
)

const (
	// defaultFetchBlockRange is the number of blocks that we collect in a single eth_getLogs query.
	defaultFetchBlockRange = uint64(100)

	// defaultPollInterval is the frequency at which we query for new rollup event.
	defaultPollInterval = 60 * time.Second

	// defaultMaxRetries is the maximum number of retries allowed when the local node is not synced up to the required block height.
	defaultMaxRetries = 20

	// defaultGetBlockInRangeRetryDelay is the time delay between retries when attempting to get blocks in range.
	// The service will wait for this duration if it detects that the local node has not synced up to the block height
	// of a specific L1 batch finalize event.
	defaultGetBlockInRangeRetryDelay = 60 * time.Second
)

// RollupSyncService collects ScrollChain batch commit/revert/finalize events and stores metadata into db.
type RollupSyncService struct {
	ctx                           context.Context
	cancel                        context.CancelFunc
	client                        *L1Client
	db                            ethdb.Database
	latestProcessedBlock          uint64
	scrollChainABI                *abi.ABI
	l1CommitBatchEventSignature   common.Hash
	l1RevertBatchEventSignature   common.Hash
	l1FinalizeBatchEventSignature common.Hash
	bc                            *core.BlockChain
	node                          *node.Node // to graceful shutdown the node
}

func NewRollupSyncService(ctx context.Context, genesisConfig *params.ChainConfig, db ethdb.Database, l1Client sync_service.EthClient, bc *core.BlockChain, node *node.Node, l1DeploymentBlock uint64) (*RollupSyncService, error) {
	// terminate if the caller does not provide an L1 client (e.g. in tests)
	if l1Client == nil || (reflect.ValueOf(l1Client).Kind() == reflect.Ptr && reflect.ValueOf(l1Client).IsNil()) {
		log.Warn("No L1 client provided, L1 rollup sync service will not run")
		return nil, nil
	}

	if genesisConfig.Scroll.L1Config == nil {
		return nil, fmt.Errorf("missing L1 config in genesis")
	}

	scrollChainABI, err := scrollChainMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("failed to get scroll chain abi: %w", err)
	}

	client, err := newL1Client(ctx, l1Client, genesisConfig.Scroll.L1Config.L1ChainId, genesisConfig.Scroll.L1Config.ScrollChainAddress, scrollChainABI)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize l1 client: %w", err)
	}

	// Initialize the latestProcessedBlock with the block just before the L1 deployment block.
	// This serves as a default value when there's no L1 rollup events synced in the database.
	var latestProcessedBlock uint64
	if l1DeploymentBlock > 0 {
		latestProcessedBlock = l1DeploymentBlock - 1
	}

	block := rawdb.ReadRollupEventSyncedL1BlockNumber(db)
	if block != nil {
		// restart from latest synced block number
		latestProcessedBlock = *block
	}

	ctx, cancel := context.WithCancel(ctx)

	service := RollupSyncService{
		ctx:                           ctx,
		cancel:                        cancel,
		client:                        client,
		db:                            db,
		latestProcessedBlock:          latestProcessedBlock,
		scrollChainABI:                scrollChainABI,
		l1CommitBatchEventSignature:   scrollChainABI.Events["CommitBatch"].ID,
		l1RevertBatchEventSignature:   scrollChainABI.Events["RevertBatch"].ID,
		l1FinalizeBatchEventSignature: scrollChainABI.Events["FinalizeBatch"].ID,
		bc:                            bc,
		node:                          node,
	}

	return &service, nil
}

func (s *RollupSyncService) Start() {
	if s == nil {
		return
	}

	log.Info("Starting rollup event sync background service", "latest processed block", s.latestProcessedBlock)

	go func() {
		t := time.NewTicker(defaultPollInterval)
		defer t.Stop()

		for {
			s.fetchRollupEvents()

			select {
			case <-s.ctx.Done():
				return
			case <-t.C:
				continue
			}
		}
	}()
}

func (s *RollupSyncService) Stop() {
	if s == nil {
		return
	}

	log.Info("Stopping rollup event sync background service")

	if s.cancel != nil {
		s.cancel()
	}
}

func (s *RollupSyncService) fetchRollupEvents() {
	latestConfirmed, err := s.client.getLatestFinalizedBlockNumber(s.ctx)
	if err != nil {
		log.Warn("failed to get latest confirmed block number", "err", err)
		return
	}

	log.Trace("Sync service fetch rollup events", "latest processed block", s.latestProcessedBlock, "latest confirmed", latestConfirmed)

	// query in batches
	for from := s.latestProcessedBlock + 1; from <= latestConfirmed; from += defaultFetchBlockRange {
		if s.ctx.Err() != nil {
			return
		}

		to := from + defaultFetchBlockRange - 1
		if to > latestConfirmed {
			to = latestConfirmed
		}

		logs, err := s.client.fetchRollupEventsInRange(s.ctx, from, to)
		if err != nil {
			log.Error("failed to fetch rollup events in range", "from block", from, "to block", to, "err", err)
			return
		}

		if err := s.parseAndUpdateRollupEventLogs(logs, to); err != nil {
			log.Error("failed to parse and update rollup event logs", "err", err)
			return
		}
	}
}

func (s *RollupSyncService) parseAndUpdateRollupEventLogs(logs []types.Log, endBlockNumber uint64) error {
	for _, vLog := range logs {
		switch vLog.Topics[0] {
		case s.l1CommitBatchEventSignature:
			event := &L1CommitBatchEvent{}
			if err := UnpackLog(s.scrollChainABI, event, "CommitBatch", vLog); err != nil {
				return fmt.Errorf("failed to unpack commit rollup event log, err: %w", err)
			}
			batchIndex := event.BatchIndex.Uint64()

			chunkBlockRanges, err := s.getChunkRanges(batchIndex, &vLog)
			if err != nil {
				return fmt.Errorf("failed to get chunk ranges, err: %w", err)
			}
			rawdb.WriteBatchChunkRanges(s.db, batchIndex, chunkBlockRanges)

		case s.l1RevertBatchEventSignature:
			event := &L1RevertBatchEvent{}
			if err := UnpackLog(s.scrollChainABI, event, "RevertBatch", vLog); err != nil {
				return fmt.Errorf("failed to unpack revert rollup event log, err: %w", err)
			}
			batchIndex := event.BatchIndex.Uint64()

			rawdb.DeleteBatchChunkRanges(s.db, batchIndex)

		case s.l1FinalizeBatchEventSignature:
			event := &L1FinalizeBatchEvent{}
			if err := UnpackLog(s.scrollChainABI, event, "FinalizeBatch", vLog); err != nil {
				return fmt.Errorf("failed to unpack finalized rollup event log, err: %w", err)
			}
			batchIndex := event.BatchIndex.Uint64()

			parentBatchMeta, chunks, err := s.getLocalInfoForBatch(batchIndex)
			if err != nil {
				return fmt.Errorf("failed to get local node info, batch index: %v, err: %w", batchIndex, err)
			}

			if err := validateBatch(event, parentBatchMeta, chunks, s.node); err != nil {
				return fmt.Errorf("fatal: validateBatch failed: finalize event: %v, err: %w", *event, err)
			}
			endChunk := chunks[len(chunks)-1]
			endBlock := endChunk.Blocks[len(endChunk.Blocks)-1]

			rawdb.WriteFinalizedL2BlockNumber(s.db, endBlock.Header.Number.Uint64())
			rawdb.WriteFinalizedBatchMeta(s.db, batchIndex, calculateFinalizedBatchMeta(parentBatchMeta, event.BatchHash, chunks))

			if batchIndex%100 == 0 {
				log.Info("finalized batch progress", "batch index", batchIndex, "finalized l2 block height", endBlock.Header.Number.Uint64())
			}

		default:
			return fmt.Errorf("unknown event, topic: %v, tx hash: %v", vLog.Topics[0].Hex(), vLog.TxHash.Hex())
		}
	}

	// note: the batch updates above are idempotent, if we crash
	// before this line and reexecute the previous steps, we will
	// get the same result.
	rawdb.WriteRollupEventSyncedL1BlockNumber(s.db, endBlockNumber)
	s.latestProcessedBlock = endBlockNumber

	return nil
}

func (s *RollupSyncService) getLocalInfoForBatch(batchIndex uint64) (*rawdb.FinalizedBatchMeta, []*Chunk, error) {
	chunkBlockRanges := rawdb.ReadBatchChunkRanges(s.db, batchIndex)
	if len(chunkBlockRanges) == 0 {
		return nil, nil, fmt.Errorf("failed to get batch chunk ranges, empty chunk block ranges")
	}

	endBlockNumber := chunkBlockRanges[len(chunkBlockRanges)-1].EndBlockNumber
	for i := 0; i < defaultMaxRetries; i++ {
		localSyncedBlockHeight := s.bc.CurrentBlock().Number().Uint64()
		if localSyncedBlockHeight >= endBlockNumber {
			break // ready to proceed, exit retry loop
		}

		log.Debug("local node is not synced up to the required block height, waiting for next retry",
			"retries", i+1, "local synced block height", localSyncedBlockHeight, "required end block number", endBlockNumber)
		time.Sleep(defaultGetBlockInRangeRetryDelay)
	}

	localSyncedBlockHeight := s.bc.CurrentBlock().Number().Uint64()
	if localSyncedBlockHeight < endBlockNumber {
		return nil, nil, fmt.Errorf("local node is not synced up to the required block height: %v, local synced block height: %v", endBlockNumber, localSyncedBlockHeight)
	}

	chunks := make([]*Chunk, len(chunkBlockRanges))
	for i, cr := range chunkBlockRanges {
		chunks[i] = &Chunk{Blocks: make([]*WrappedBlock, cr.EndBlockNumber-cr.StartBlockNumber+1)}
		for j := cr.StartBlockNumber; j <= cr.EndBlockNumber; j++ {
			block := s.bc.GetBlockByNumber(j)
			if block == nil {
				return nil, nil, fmt.Errorf("failed to get block by number: %v", i)
			}
			txData := txsToTxsData(block.Transactions())
			state, err := s.bc.StateAt(block.Root())
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get block state, block: %v, err: %w", block.Hash().Hex(), err)
			}
			withdrawRoot := withdrawtrie.ReadWTRSlot(rcfg.L2MessageQueueAddress, state)
			chunks[i].Blocks[j-cr.StartBlockNumber] = &WrappedBlock{
				Header:       block.Header(),
				Transactions: txData,
				WithdrawRoot: withdrawRoot,
			}
		}
	}

	// get metadata of parent batch: default to genesis batch metadata.
	parentBatchMeta := &rawdb.FinalizedBatchMeta{}
	if batchIndex > 0 {
		parentBatchMeta = rawdb.ReadFinalizedBatchMeta(s.db, batchIndex-1)
	}

	return parentBatchMeta, chunks, nil
}

func (s *RollupSyncService) getChunkRanges(batchIndex uint64, vLog *types.Log) ([]*rawdb.ChunkBlockRange, error) {
	if batchIndex == 0 {
		return []*rawdb.ChunkBlockRange{{StartBlockNumber: 0, EndBlockNumber: 0}}, nil
	}

	tx, _, err := s.client.client.TransactionByHash(context.Background(), vLog.TxHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction, err: %w", err)
	}

	return s.decodeChunkBlockRanges(tx.Data())
}

// decodeChunkBlockRanges decodes chunks in a batch based on the commit batch transaction's calldata.
func (s *RollupSyncService) decodeChunkBlockRanges(txData []byte) ([]*rawdb.ChunkBlockRange, error) {
	const methodIDLength = 4
	if len(txData) < methodIDLength {
		return nil, fmt.Errorf("transaction data is too short")
	}

	method, err := s.scrollChainABI.MethodById(txData[:4])
	if err != nil {
		return nil, fmt.Errorf("failed to get method by ID, ID: %v, err: %w", txData[:4], err)
	}

	decoded, err := method.Inputs.Unpack(txData[4:])
	if err != nil {
		return nil, fmt.Errorf("failed to unpack transaction data using ABI: %v", err)
	}

	const expectedLength = 4
	if len(decoded) != expectedLength {
		return nil, fmt.Errorf("invalid decoded length, expected: %d, got: %v", expectedLength, len(decoded))
	}

	version, ok := decoded[0].(uint8)
	if !ok {
		return nil, fmt.Errorf("failed to cast version to uint8")
	}
	if version != batchHeaderVersion {
		return nil, fmt.Errorf("unexpected batch version, expected: %d, got: %v", batchHeaderVersion, version)
	}

	chunks, ok := decoded[2].([][]byte)
	if !ok {
		return nil, fmt.Errorf("failed to cast chunks to slice of byte slices")
	}

	return DecodeChunkBlockRanges(chunks)
}

func calculateFinalizedBatchMeta(parentBatchMeta *rawdb.FinalizedBatchMeta, batchHash common.Hash, chunks []*Chunk) rawdb.FinalizedBatchMeta {
	totalL1MessagePopped := parentBatchMeta.TotalL1MessagePopped
	for _, chunk := range chunks {
		totalL1MessagePopped += chunk.NumL1Messages(totalL1MessagePopped)
	}
	return rawdb.FinalizedBatchMeta{
		BatchHash:            batchHash,
		TotalL1MessagePopped: totalL1MessagePopped,
	}
}

// validateBatch verifies the consistency between l1 contract and l2 node data.
// close the node and exit once any consistency check fails.
func validateBatch(event *L1FinalizeBatchEvent, parentBatchMeta *rawdb.FinalizedBatchMeta, chunks []*Chunk, node *node.Node) error {
	if len(chunks) == 0 {
		return fmt.Errorf("invalid arg: length of chunks is 0")
	}
	endChunk := chunks[len(chunks)-1]
	if len(endChunk.Blocks) == 0 {
		return fmt.Errorf("invalid arg: block number of last chunk is 0")
	}
	endBlock := endChunk.Blocks[len(endChunk.Blocks)-1]
	localWithdrawRoot := endBlock.WithdrawRoot
	if localWithdrawRoot != event.WithdrawRoot {
		log.Error("Withdraw root mismatch", "l1 withdraw root", event.WithdrawRoot.Hex(), "l2 withdraw root", localWithdrawRoot.Hex())
		node.Close()
		os.Exit(1)
	}

	localStateRoot := endBlock.Header.Root
	if localStateRoot != event.StateRoot {
		log.Error("State root mismatch", "l1 state root", event.StateRoot.Hex(), "l2 state root", localStateRoot.Hex())
		node.Close()
		os.Exit(1)
	}

	batchHeader, err := NewBatchHeader(batchHeaderVersion, event.BatchIndex.Uint64(), parentBatchMeta.TotalL1MessagePopped, parentBatchMeta.BatchHash, chunks)
	if err != nil {
		return fmt.Errorf("failed to construct batch header, err: %w", err)
	}

	localBatchHash := batchHeader.Hash()
	if localBatchHash != event.BatchHash {
		log.Error("Batch hash mismatch", "l1 batch hash", event.BatchHash.Hex(), "l2 batch hash", localBatchHash.Hex())
		node.Close()
		os.Exit(1)
	}

	return nil
}
