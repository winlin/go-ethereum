package eventwatcher

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
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

	// defaultPollInterval is the frequency at which we query for new batch event.
	defaultPollInterval = time.Second * 60

	// logProgressInterval is the frequency at which we log progress.
	logProgressInterval = time.Second * 600
)

// EventWatcher collects ScrollChain batch commit/revert/finalize events and stores metadata into db.
type EventWatcher struct {
	ctx                           context.Context
	cancel                        context.CancelFunc
	client                        *L1Client
	db                            ethdb.Database
	pollInterval                  time.Duration
	latestProcessedBlock          uint64
	scrollChainABI                *abi.ABI
	l1CommitBatchEventSignature   common.Hash
	l1RevertBatchEventSignature   common.Hash
	l1FinalizeBatchEventSignature common.Hash
	bc                            *core.BlockChain
}

func NewEventWatcher(ctx context.Context, genesisConfig *params.ChainConfig, nodeConfig *node.Config, db ethdb.Database, l1Client sync_service.EthClient, bc *core.BlockChain) (*EventWatcher, error) {
	// terminate if the caller does not provide an L1 client (e.g. in tests)
	if l1Client == nil || (reflect.ValueOf(l1Client).Kind() == reflect.Ptr && reflect.ValueOf(l1Client).IsNil()) {
		log.Warn("No L1 client provided, L1 sync service will not run")
		return nil, nil
	}

	if genesisConfig.Scroll.L1Config == nil {
		return nil, fmt.Errorf("missing L1 config in genesis")
	}

	scrollChainABI, err := scrollChainMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("failed to get scroll chain abi: %w", err)
	}

	// use "finalized" tag for l1 confirmations, otherwise reorg may cause data mismatch.
	client, err := newL1Client(ctx, l1Client, genesisConfig.Scroll.L1Config.L1ChainId, nodeConfig.L1Confirmations,
		genesisConfig.Scroll.L1Config.ScrollChainAddress, scrollChainABI)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize l1 client: %w", err)
	}

	// if no data in db, which indicates not synced l1 batch events successfully yet.
	// then eventwatcher would sync from the l1 deployment block (inclusive).
	latestProcessedBlock := nodeConfig.L1DeploymentBlock - 1
	block := rawdb.ReadBatchEventSyncedL1BlockNumber(db)
	if block != nil {
		// restart from latest synced block number
		latestProcessedBlock = *block
	}

	ctx, cancel := context.WithCancel(ctx)

	service := EventWatcher{
		ctx:                           ctx,
		cancel:                        cancel,
		client:                        client,
		db:                            db,
		pollInterval:                  defaultPollInterval,
		latestProcessedBlock:          latestProcessedBlock,
		scrollChainABI:                scrollChainABI,
		l1CommitBatchEventSignature:   scrollChainABI.Events["CommitBatch"].ID,
		l1RevertBatchEventSignature:   scrollChainABI.Events["RevertBatch"].ID,
		l1FinalizeBatchEventSignature: scrollChainABI.Events["FinalizeBatch"].ID,
		bc:                            bc,
	}

	return &service, nil
}

func (s *EventWatcher) Start() {
	if s == nil {
		return
	}

	log.Info("Starting batch event sync background service", "latest processed block", s.latestProcessedBlock)

	go func() {
		t := time.NewTicker(s.pollInterval)
		defer t.Stop()

		for {
			s.fetchBatchEvents()

			select {
			case <-s.ctx.Done():
				return
			case <-t.C:
				continue
			}
		}
	}()
}

func (s *EventWatcher) Stop() {
	if s == nil {
		return
	}

	log.Info("Stopping batch event sync background service")

	if s.cancel != nil {
		s.cancel()
	}
}

func (s *EventWatcher) fetchBatchEvents() {
	latestConfirmed, err := s.client.getLatestFinalizedBlockNumber(s.ctx)
	if err != nil {
		log.Warn("failed to get latest confirmed block number", "err", err)
		return
	}

	log.Trace("Sync service fetchBatchEvents", "latestProcessedBlock", s.latestProcessedBlock, "latestConfirmed", latestConfirmed)

	// ticker for logging progress
	t := time.NewTicker(logProgressInterval)

	// query in batches
	for from := s.latestProcessedBlock + 1; from <= latestConfirmed; from += defaultFetchBlockRange {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
			progress := 100 * float64(s.latestProcessedBlock) / float64(latestConfirmed)
			log.Info("Syncing batch events", "processed", s.latestProcessedBlock, "confirmed", latestConfirmed, "progress(%)", progress)
		default:
		}

		to := from + defaultFetchBlockRange - 1
		if to > latestConfirmed {
			to = latestConfirmed
		}

		logs, err := s.client.fetchBatchEventsInRange(s.ctx, from, to)
		if err != nil {
			// return and retry in next loop
			log.Warn("failed to fetch batch events in range", "fromBlock", from, "toBlock", to, "err", err)
			return
		}

		if err := s.parseAndUpdateBatchEventLogs(logs, to); err != nil {
			log.Error("failed to parse and update batch event logs", "err", err)
		}
	}
}

func (s *EventWatcher) parseAndUpdateBatchEventLogs(logs []types.Log, lastBlock uint64) error {
	for _, vLog := range logs {
		switch vLog.Topics[0] {
		case s.l1CommitBatchEventSignature:
			event := L1CommitBatchEvent{}
			if err := UnpackLog(s.scrollChainABI, event, "CommitBatch", vLog); err != nil {
				return fmt.Errorf("failed to unpack commit batch event log, err: %w", err)
			}
			batchIndex := vLog.Topics[1].Big().Uint64()
			chunkRanges, err := s.getChunkRanges(batchIndex, &vLog)
			if err != nil {
				return fmt.Errorf("failed to get chunk ranges, err: %w", err)
			}
			rawdb.WriteBatchChunkRanges(s.db, batchIndex, chunkRanges)

		case s.l1RevertBatchEventSignature:
			event := L1RevertBatchEvent{}
			if err := UnpackLog(s.scrollChainABI, event, "RevertBatch", vLog); err != nil {
				return fmt.Errorf("failed to unpack revert batch event log, err: %w", err)
			}
			batchIndex := vLog.Topics[1].Big().Uint64()
			rawdb.DeleteBatchChunkRanges(s.db, batchIndex)

		case s.l1FinalizeBatchEventSignature:
			event := L1FinalizeBatchEvent{}
			if err := UnpackLog(s.scrollChainABI, event, "FinalizeBatch", vLog); err != nil {
				return fmt.Errorf("failed to unpack finalized batch event log, err: %w", err)
			}
			batchIndex := event.BatchIndex.Uint64()
			batchHash := event.BatchHash
			stateRoot := event.StateRoot
			withdrawRoot := event.WithdrawRoot

			parentBatchMeta, chunks, err := s.getLocalInfo(batchIndex)
			if err != nil {
				return fmt.Errorf("failed to get local node info, batch index: %v, err: %w", batchIndex, err)
			}

			if err := reconciliation(batchIndex, batchHash, stateRoot, withdrawRoot, parentBatchMeta, chunks); err != nil {
				return fmt.Errorf("fatal: reconciliation failed: batch index: %v, err: %w", batchIndex, err)
			}
			rawdb.WriteFinalizedL2BlockNumber(s.db, batchIndex)
			rawdb.WriteFinalizedBatchMeta(s.db, batchIndex, s.getFinalizedBatchMeta(batchHash, parentBatchMeta, chunks))

		default:
			return fmt.Errorf("unknown event, topic: %v, tx hash: %v", vLog.Topics[0].Hex(), vLog.TxHash.Hex())
		}
	}

	rawdb.WriteBatchEventSyncedL1BlockNumber(s.db, lastBlock)
	s.latestProcessedBlock = lastBlock

	return nil
}

func (s *EventWatcher) getFinalizedBatchMeta(batchHash common.Hash, parentBatchMeta *rawdb.FinalizedBatchMeta, chunks []*Chunk) rawdb.FinalizedBatchMeta {
	totalL1MessagePopped := parentBatchMeta.TotalL1MessagePopped
	for _, chunk := range chunks {
		totalL1MessagePopped += chunk.NumL1Messages(totalL1MessagePopped)
	}
	return rawdb.FinalizedBatchMeta{
		BatchHash:            batchHash,
		TotalL1MessagePopped: totalL1MessagePopped,
	}
}

func (s *EventWatcher) getLocalInfo(batchIndex uint64) (*rawdb.FinalizedBatchMeta, []*Chunk, error) {
	chunkRanges := rawdb.ReadBatchChunkRanges(s.db, batchIndex)
	blocks, err := s.getBlocksInRange(chunkRanges)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get blocks in range, err: %w", err)
	}

	// default to genesis batch meta.
	parentBatchMeta := &rawdb.FinalizedBatchMeta{}
	if batchIndex > 0 {
		parentBatchMeta = rawdb.ReadFinalizedBatchMeta(s.db, batchIndex-1)
	}

	chunks, err := s.convertBlocksToChunks(blocks, chunkRanges)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert blocks to chunks, batch index: %v, chunk ranges: %v, err: %w", batchIndex, chunkRanges, err)
	}
	return parentBatchMeta, chunks, nil
}

func (s *EventWatcher) getChunkRanges(batchIndex uint64, vLog *types.Log) ([]*rawdb.ChunkRange, error) {
	if batchIndex == 0 {
		return []*rawdb.ChunkRange{{StartBlockNumber: 0, EndBlockNumber: 0}}, nil
	}

	tx, _, err := s.client.client.TransactionByHash(context.Background(), vLog.TxHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction, err: %w", err)
	}

	return s.decodeChunkRanges(tx.Data())
}

func (s *EventWatcher) decodeChunkRanges(txData []byte) ([]*rawdb.ChunkRange, error) {
	decoded, err := s.scrollChainABI.Unpack("commitBatch", txData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode transaction data, err: %w", err)
	}

	if len(decoded) != 4 {
		return nil, fmt.Errorf("invalid decoded length, expected: 4, got: %v,", len(decoded))
	}

	chunks := decoded[2].([]string)
	var chunkRanges []*rawdb.ChunkRange
	startBlockNumber, err := strconv.ParseUint(chunks[0][4:20], 16, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse blockNumber, err: %w", err)
	}

	for _, chunk := range chunks {
		numBlocks, err := strconv.ParseUint(chunk[0:4], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("failed to parse numBlocks, err: %w", err)
		}
		chunkRanges = append(chunkRanges, &rawdb.ChunkRange{
			StartBlockNumber: startBlockNumber,
			EndBlockNumber:   startBlockNumber + numBlocks - 1,
		})
		startBlockNumber += numBlocks
	}

	return chunkRanges, nil
}

// getBlocksInRange retrieves blocks from the blockchain within specified chunk ranges.
func (s *EventWatcher) getBlocksInRange(chunkRanges []*rawdb.ChunkRange) ([]*types.Block, error) {
	var blocks []*types.Block

	for _, chunkRange := range chunkRanges {
		for i := chunkRange.StartBlockNumber; i <= chunkRange.EndBlockNumber; i++ {
			block := s.bc.GetBlockByNumber(i)
			if block == nil {
				return nil, fmt.Errorf("failed to get block by number: %v", i)
			}
			blocks = append(blocks, block)
		}
	}

	return blocks, nil
}

// convertBlocksToChunks processes and groups blocks into chunks based on provided chunk ranges.
func (s *EventWatcher) convertBlocksToChunks(blocks []*types.Block, chunkRanges []*rawdb.ChunkRange) ([]*Chunk, error) {
	if len(blocks) == 0 {
		return nil, fmt.Errorf("invalid arg: empty blocks")
	}

	wrappedBlocks := make([]*WrappedBlock, len(blocks))
	for i, block := range blocks {
		txData := txsToTxsData(block.Transactions())
		state, err := s.bc.StateAt(block.Hash())
		if err != nil {
			return nil, fmt.Errorf("failed to get block state, block: %v, err: %w", block.Hash().Hex(), err)
		}
		withdrawRoot := withdrawtrie.ReadWTRSlot(rcfg.L2MessageQueueAddress, state)
		wrappedBlocks[i] = &WrappedBlock{
			Header:       block.Header(),
			Transactions: txData,
			WithdrawRoot: withdrawRoot,
		}
	}

	minBlockNumber := blocks[0].Header().Number.Uint64()
	var chunks []*Chunk
	for _, cr := range chunkRanges {
		start, end := cr.StartBlockNumber-minBlockNumber, cr.EndBlockNumber-minBlockNumber
		// ensure start and end are within valid range.
		if start < 0 || end >= uint64(len(wrappedBlocks)) || start > end {
			return nil, fmt.Errorf("invalid chunk range, start: %v, end: %v, block len: %v", start, end, len(wrappedBlocks))
		}
		chunk := &Chunk{
			Blocks: wrappedBlocks[start : end+1],
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// reconciliation verifies the consistency between l1 contract and l2 node data.
// crash once the consistency check fails.
func reconciliation(batchIndex uint64, batchHash common.Hash, stateRoot common.Hash, withdrawRoot common.Hash, parentBatchMeta *rawdb.FinalizedBatchMeta, chunks []*Chunk) error {
	if len(chunks) == 0 {
		return fmt.Errorf("invalid arg: length of chunks is 0")
	}
	lastChunk := chunks[len(chunks)-1]
	if len(lastChunk.Blocks) == 0 {
		return fmt.Errorf("invalid arg: block number of last chunk is 0")
	}
	lastBlock := lastChunk.Blocks[len(lastChunk.Blocks)-1]
	localWithdrawRoot := lastBlock.WithdrawRoot
	if localWithdrawRoot != withdrawRoot {
		log.Crit("Withdraw root mismatch", "l1 withdraw root", withdrawRoot.Hex(), "l2 withdraw root", localWithdrawRoot.Hex())
	}

	localStateRoot := lastBlock.Header.Root
	if localStateRoot != stateRoot {
		log.Crit("State root mismatch", "l1 state root", stateRoot.Hex(), "l2 state root", localStateRoot.Hex())
	}

	batchHeader, err := NewBatchHeader(batchHeaderVersion, batchIndex, parentBatchMeta.TotalL1MessagePopped, parentBatchMeta.BatchHash, chunks)
	if err != nil {
		return fmt.Errorf("failed to construct batch header, err: %w", err)
	}

	localBatchHash := batchHeader.Hash()
	if localBatchHash != batchHash {
		log.Crit("Batch hash mismatch", "l1 batch hash", batchHash.Hex(), "l2 batch hash", localBatchHash.Hex())
	}

	return nil
}
