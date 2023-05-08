package zktrie

import (
	"math/big"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/syndtr/goleveldb/leveldb"

	zktrie "github.com/scroll-tech/zktrie/trie"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/ethdb"
	"github.com/scroll-tech/go-ethereum/metrics"
	"github.com/scroll-tech/go-ethereum/trie"
)

var (
	memcacheCleanHitMeter   = metrics.NewRegisteredMeter("zktrie/memcache/clean/hit", nil)
	memcacheCleanMissMeter  = metrics.NewRegisteredMeter("zktrie/memcache/clean/miss", nil)
	memcacheCleanReadMeter  = metrics.NewRegisteredMeter("zktrie/memcache/clean/read", nil)
	memcacheCleanWriteMeter = metrics.NewRegisteredMeter("zktrie/memcache/clean/write", nil)

	memcacheDirtyHitMeter   = metrics.NewRegisteredMeter("zktrie/memcache/dirty/hit", nil)
	memcacheDirtyMissMeter  = metrics.NewRegisteredMeter("zktrie/memcache/dirty/miss", nil)
	memcacheDirtyReadMeter  = metrics.NewRegisteredMeter("zktrie/memcache/dirty/read", nil)
	memcacheDirtyWriteMeter = metrics.NewRegisteredMeter("zktrie/memcache/dirty/write", nil)

	memcacheFlushTimeTimer  = metrics.NewRegisteredResettingTimer("zktrie/memcache/flush/time", nil)
	memcacheFlushNodesMeter = metrics.NewRegisteredMeter("zktrie/memcache/flush/nodes", nil)
	memcacheFlushSizeMeter  = metrics.NewRegisteredMeter("zktrie/memcache/flush/size", nil)

	memcacheGCTimeTimer  = metrics.NewRegisteredResettingTimer("zktrie/memcache/gc/time", nil)
	memcacheGCNodesMeter = metrics.NewRegisteredMeter("zktrie/memcache/gc/nodes", nil)
	memcacheGCSizeMeter  = metrics.NewRegisteredMeter("zktrie/memcache/gc/size", nil)

	memcacheCommitTimeTimer  = metrics.NewRegisteredResettingTimer("zktrie/memcache/commit/time", nil)
	memcacheCommitNodesMeter = metrics.NewRegisteredMeter("zktrie/memcache/commit/nodes", nil)
	memcacheCommitSizeMeter  = metrics.NewRegisteredMeter("zktrie/memcache/commit/size", nil)
)

var (
	cachedNodeSize = int(reflect.TypeOf(trie.KV{}).Size())
)

// Database Database adaptor imple zktrie.ZktrieDatbase
type Database struct {
	diskdb ethdb.KeyValueStore // Persistent storage for matured trie nodes
	prefix []byte

	cleans     *fastcache.Cache // GC friendly memory cache of clean node RLPs
	rawDirties trie.KvMap

	preimages *preimageStore

	lock sync.RWMutex
}

// Config defines all necessary options for database.
type Config struct {
	Cache     int  // Memory allowance (MB) to use for caching trie nodes in memory
	Preimages bool // Flag whether the preimage of trie key is recorded
}

func NewDatabase(diskdb ethdb.KeyValueStore) *Database {
	return NewDatabaseWithConfig(diskdb, nil)
}

func NewDatabaseWithConfig(diskdb ethdb.KeyValueStore, config *Config) *Database {
	var cleans *fastcache.Cache
	if config != nil && config.Cache > 0 {
		cleans = fastcache.New(config.Cache * 1024 * 1024)
	}
	db := &Database{
		diskdb:     diskdb,
		prefix:     []byte{},
		cleans:     cleans,
		rawDirties: make(trie.KvMap),
	}
	if config != nil && config.Preimages {
		db.preimages = newPreimageStore(diskdb)
	}
	return db
}

// Put saves a key:value into the Storage
func (db *Database) Put(k, v []byte) error {
	db.lock.Lock()
	db.rawDirties.Put(trie.Concat(db.prefix, k[:]), v)
	db.lock.Unlock()
	return nil
}

// Get retrieves a value from a key in the Storage
func (db *Database) Get(key []byte) ([]byte, error) {
	concatKey := trie.Concat(db.prefix, key[:])
	db.lock.RLock()
	value, ok := db.rawDirties.Get(concatKey)
	db.lock.RUnlock()
	if ok {
		return value, nil
	}

	if db.cleans != nil {
		if enc := db.cleans.Get(nil, concatKey); enc != nil {
			memcacheCleanHitMeter.Mark(1)
			memcacheCleanReadMeter.Mark(int64(len(enc)))
			return enc, nil
		}
	}

	v, err := db.diskdb.Get(concatKey)
	if err == leveldb.ErrNotFound {
		return nil, zktrie.ErrKeyNotFound
	}
	if db.cleans != nil {
		db.cleans.Set(concatKey[:], v)
		memcacheCleanMissMeter.Mark(1)
		memcacheCleanWriteMeter.Mark(int64(len(v)))
	}
	return v, err
}

func (db *Database) UpdatePreimage(preimage []byte, hashField *big.Int) {
	if db.preimages != nil { // Ugly direct check but avoids the below write lock
		// we must copy the input key
		db.preimages.insertPreimage(map[common.Hash][]byte{common.BytesToHash(hashField.Bytes()): common.CopyBytes(preimage)})
	}
}

// Iterate implements the method Iterate of the interface Storage
func (db *Database) Iterate(f func([]byte, []byte) (bool, error)) error {
	iter := db.diskdb.NewIterator(db.prefix, nil)
	defer iter.Release()
	for iter.Next() {
		localKey := iter.Key()[len(db.prefix):]
		if cont, err := f(localKey, iter.Value()); err != nil {
			return err
		} else if !cont {
			break
		}
	}
	iter.Release()
	return iter.Error()
}

func (db *Database) Reference(child common.Hash, parent common.Hash) {
	panic("not implemented")
}

func (db *Database) Dereference(root common.Hash) {
	panic("not implemented")
}

// Close implements the method Close of the interface Storage
func (db *Database) Close() {
	// FIXME: is this correct?
	if err := db.diskdb.Close(); err != nil {
		panic(err)
	}
}

// List implements the method List of the interface Storage
func (db *Database) List(limit int) ([]trie.KV, error) {
	ret := []trie.KV{}
	err := db.Iterate(func(key []byte, value []byte) (bool, error) {
		ret = append(ret, trie.KV{K: trie.Clone(key), V: trie.Clone(value)})
		if len(ret) == limit {
			return false, nil
		}
		return true, nil
	})
	return ret, err
}

func (db *Database) Commit(node common.Hash, report bool, callback func(common.Hash)) error {
	batch := db.diskdb.NewBatch()

	db.lock.Lock()
	for _, v := range db.rawDirties {
		batch.Put(v.K, v.V)
	}
	for k := range db.rawDirties {
		delete(db.rawDirties, k)
	}
	db.lock.Unlock()
	if err := batch.Write(); err != nil {
		return err
	}
	batch.Reset()
	return nil
}

// DiskDB retrieves the persistent storage backing the trie database.
func (db *Database) DiskDB() ethdb.KeyValueStore {
	return db.diskdb
}

// EmptyRoot indicate what root is for an empty trie
func (db *Database) EmptyRoot() common.Hash {
	return emptyRoot
}

// saveCache saves clean state cache to given directory path
// using specified CPU cores.
func (db *Database) saveCache(dir string, threads int) error {
	//TODO: impelement it?
	return nil
}

// SaveCachePeriodically atomically saves fast cache data to the given dir with
// the specified interval. All dump operation will only use a single CPU core.
func (db *Database) SaveCachePeriodically(dir string, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			db.saveCache(dir, 1)
		case <-stopCh:
			return
		}
	}
}

func (db *Database) Size() (common.StorageSize, common.StorageSize) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	return common.StorageSize(len(db.rawDirties) * cachedNodeSize), db.preimages.size()
}

func (db *Database) SaveCache(dir string) error {
	return db.saveCache(dir, runtime.GOMAXPROCS(0))
}

func (db *Database) Node(hash common.Hash) ([]byte, error) {
	panic("not implemented")
}

// Cap iteratively flushes old but still referenced trie nodes until the total
// memory usage goes below the given threshold.
//
// Note, this method is a non-synchronized mutator. It is unsafe to call this
// concurrently with other mutators.
func (db *Database) Cap(size common.StorageSize) {
	panic("not implemented")
}
