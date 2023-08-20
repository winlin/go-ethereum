package rawdb

import (
	"encoding/binary"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/ethdb"
	"github.com/scroll-tech/go-ethereum/log"
)

func WriteNextReplayIndex(db ethdb.KeyValueWriter, parentHash common.Hash, replayIndex uint64) {
	if err := db.Put(NextReplayIndexKey(parentHash), encodeQueueIndex(replayIndex)); err != nil {
		log.Crit("Failed to store first next replay index", "parentHash", parentHash, "err", err)
	}
}

func ReadNextReplayIndex(db ethdb.Reader, parentHash common.Hash) uint64 {
	data, err := db.Get(NextReplayIndexKey(parentHash))
	if err != nil && isNotFoundErr(err) {
		return 0
	}
	if err != nil {
		log.Crit("Failed to read first next replay index from database", "parentHash", parentHash, "err", err)
	}
	if len(data) == 0 {
		return 0
	}
	return binary.BigEndian.Uint64(data)
}
