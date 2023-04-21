// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package zktrie

import (
	"errors"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/common/prque"
	"github.com/scroll-tech/go-ethereum/ethdb"
)

// ErrNotRequested is returned by the trie sync when it's requested to process a
// node it did not request.
var ErrNotRequested = errors.New("not requested")

// ErrAlreadyProcessed is returned by the trie sync when it's requested to process a
// node it already processed previously.
var ErrAlreadyProcessed = errors.New("already processed")

// maxFetchesPerDepth is the maximum number of pending trie nodes per depth. The
// role of this value is to limit the number of trie nodes that get expanded in
// memory if the node was configured with a significant number of peers.
const maxFetchesPerDepth = 16384

// request represents a scheduled or already in-flight state retrieval request.
type request struct {
	path []byte      // Merkle path leading to this node for prioritization
	hash common.Hash // Hash of the node data content to retrieve
	data []byte      // Data content of the node, cached until all subtrees complete
	code bool        // Whether this is a code entry

	parents []*request // Parent state nodes referencing this entry (notify all upon completion)
	deps    int        // Number of dependencies before allowed to commit this node

	callback LeafCallback // Callback to invoke if a leaf node it reached on this branch
}

// SyncPath is a path tuple identifying a particular trie node either in a single
// trie (account) or a layered trie (account -> storage).
//
// Content wise the tuple either has 1 element if it addresses a node in a single
// trie or 2 elements if it addresses a node in a stacked trie.
//
// To support aiming arbitrary trie nodes, the path needs to support odd nibble
// lengths. To avoid transferring expanded hex form over the network, the last
// part of the tuple (which needs to index into the middle of a trie) is compact
// encoded. In case of a 2-tuple, the first item is always 32 bytes so that is
// simple binary encoded.
//
// Examples:
//   - Path 0x9  -> {0x19}
//   - Path 0x99 -> {0x0099}
//   - Path 0x01234567890123456789012345678901012345678901234567890123456789019  -> {0x0123456789012345678901234567890101234567890123456789012345678901, 0x19}
//   - Path 0x012345678901234567890123456789010123456789012345678901234567890199 -> {0x0123456789012345678901234567890101234567890123456789012345678901, 0x0099}
type SyncPath [][]byte

// newSyncPath converts an expanded trie path from nibble form into a compact
// version that can be sent over the network.
func newSyncPath(path []byte) SyncPath {
	panic("not implemented")
	// If the hash is from the account trie, append a single item, if it
	// is from the a storage trie, append a tuple. Note, the length 64 is
	// clashing between account leaf and storage root. It's fine though
	// because having a trie node at 64 depth means a hash collision was
	// found and we're long dead.
	//if len(path) < 64 {
	//	return SyncPath{hexToCompact(path)}
	//}
	//return SyncPath{hexToKeybytes(path[:64]), hexToCompact(path[64:])}
}

// SyncResult is a response with requested data along with it's hash.
type SyncResult struct {
	Hash common.Hash // Hash of the originally unknown trie node
	Data []byte      // Data content of the retrieved node
}

// syncMemBatch is an in-memory buffer of successfully downloaded but not yet
// persisted data items.
type syncMemBatch struct {
	nodes map[common.Hash][]byte // In-memory membatch of recently completed nodes
	codes map[common.Hash][]byte // In-memory membatch of recently completed codes
}

// newSyncMemBatch allocates a new memory-buffer for not-yet persisted trie nodes.
func newSyncMemBatch() *syncMemBatch {
	return &syncMemBatch{
		nodes: make(map[common.Hash][]byte),
		codes: make(map[common.Hash][]byte),
	}
}

// hasNode reports the trie node with specific hash is already cached.
func (batch *syncMemBatch) hasNode(hash common.Hash) bool {
	_, ok := batch.nodes[hash]
	return ok
}

// hasCode reports the contract code with specific hash is already cached.
func (batch *syncMemBatch) hasCode(hash common.Hash) bool {
	_, ok := batch.codes[hash]
	return ok
}

// Sync is the main state trie synchronisation scheduler, which provides yet
// unknown trie hashes to retrieve, accepts node data associated with said hashes
// and reconstructs the trie step by step until all is done.
type Sync struct {
	database ethdb.KeyValueReader     // Persistent database to check for existing entries
	membatch *syncMemBatch            // Memory buffer to avoid frequent database writes
	nodeReqs map[common.Hash]*request // Pending requests pertaining to a trie node hash
	codeReqs map[common.Hash]*request // Pending requests pertaining to a code hash
	queue    *prque.Prque             // Priority queue with the pending requests
	fetches  map[int]int              // Number of active fetches per trie node depth
	bloom    *SyncBloom               // Bloom filter for fast state existence checks
}

// NewSync creates a new trie data download scheduler.
func NewSync(root common.Hash, database ethdb.KeyValueReader, callback LeafCallback, bloom *SyncBloom) *Sync {
	ts := &Sync{
		database: database,
		membatch: newSyncMemBatch(),
		nodeReqs: make(map[common.Hash]*request),
		codeReqs: make(map[common.Hash]*request),
		queue:    prque.New(nil),
		fetches:  make(map[int]int),
		bloom:    bloom,
	}
	ts.AddSubTrie(root, nil, common.Hash{}, callback)
	return ts
}

// AddSubTrie registers a new trie to the sync code, rooted at the designated parent.
func (s *Sync) AddSubTrie(root common.Hash, path []byte, parent common.Hash, callback LeafCallback) {
	panic("not implemented")
}

// AddCodeEntry schedules the direct retrieval of a contract code that should not
// be interpreted as a trie node, but rather accepted and stored into the database
// as is.
func (s *Sync) AddCodeEntry(hash common.Hash, path []byte, parent common.Hash) {
	panic("not implemented")
}

// Missing retrieves the known missing nodes from the trie for retrieval. To aid
// both eth/6x style fast sync and snap/1x style state sync, the paths of trie
// nodes are returned too, as well as separate hash list for codes.
func (s *Sync) Missing(max int) (nodes []common.Hash, paths []SyncPath, codes []common.Hash) {
	panic("not implemented")
}

// Process injects the received data for requested item. Note it can
// happpen that the single response commits two pending requests(e.g.
// there are two requests one for code and one for node but the hash
// is same). In this case the second response for the same hash will
// be treated as "non-requested" item or "already-processed" item but
// there is no downside.
func (s *Sync) Process(result SyncResult) error {
	panic("not implemented")
}

// Commit flushes the data stored in the internal membatch out to persistent
// storage, returning any occurred error.
func (s *Sync) Commit(dbw ethdb.Batch) error {
	panic("not implemented")
}

// Pending returns the number of state entries currently pending for download.
func (s *Sync) Pending() int {
	return len(s.nodeReqs) + len(s.codeReqs)
}
