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
	"fmt"
	"reflect"
	"unsafe"

	itrie "github.com/scroll-tech/zktrie/trie"
	itypes "github.com/scroll-tech/zktrie/types"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/log"
	"github.com/scroll-tech/go-ethereum/trie"
)

var (
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.Hash{}

	//TODO
	// emptyState is the known hash of an empty state trie entry.
	emptyState = common.HexToHash("implement me!!")
)

// LeafCallback is a callback type invoked when a trie operation reaches a leaf
// node.
//
// The paths is a path tuple identifying a particular trie node either in a single
// trie (account) or a layered trie (account -> storage). Each path in the tuple
// is in the raw format(32 bytes).
//
// The hexpath is a composite hexary path identifying the trie node. All the key
// bytes are converted to the hexary nibbles and composited with the parent path
// if the trie node is in a layered trie.
//
// It's used by state sync and commit to allow handling external references
// between account and storage tries. And also it's used in the state healing
// for extracting the raw states(leaf nodes) with corresponding paths.
type LeafCallback func(paths [][]byte, hexpath []byte, leaf []byte, parent common.Hash) error

type Trie struct {
	db   *Database
	impl *itrie.ZkTrieImpl
	// tr is constructed for ZkTrie.ProofWithDeletion calling
	tr *itrie.ZkTrie
}

func unsafeSetImpl(zkTrie *itrie.ZkTrie, impl *itrie.ZkTrieImpl) {
	implField := reflect.ValueOf(zkTrie).Elem().Field(0)
	implField = reflect.NewAt(implField.Type(), unsafe.Pointer(implField.UnsafeAddr())).Elem()
	implField.Set(reflect.ValueOf(impl))
}

// New creates a trie
// New bypasses all the buffer mechanism in *Database, it directly uses the
// underlying diskdb
func New(root common.Hash, db *Database) (*Trie, error) {
	if db == nil {
		panic("zktrie.New called without a database")
	}

	// for proof generation
	impl, err := itrie.NewZkTrieImplWithRoot(db, zktNodeHash(root), itrie.NodeKeyValidBytes*8)
	if err != nil {
		return nil, err
	}

	tr := &itrie.ZkTrie{}
	//TODO: it is ugly and dangerous, fix it in the zktrie repo later!
	unsafeSetImpl(tr, impl)

	return &Trie{impl: impl, tr: tr, db: db}, nil
}

// Get returns the value for key stored in the trie.
// The value bytes must not be modified by the caller.
func (t *Trie) Get(key []byte) []byte {
	res, err := t.impl.TryGet(bytesToHash(key))
	if err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
	return res
}

func (t *Trie) TryGet(key []byte) ([]byte, error) {
	return t.impl.TryGet(bytesToHash(key))
}

// Update associates key with value in the trie. Subsequent calls to
// Get will return value. If value has length zero, any existing value
// is deleted from the trie and calls to Get will return nil.
//
// The value bytes must not be modified by the caller while they are
// stored in the trie.
func (t *Trie) Update(key, value []byte) {
	if err := t.TryUpdate(key, value); err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
}

func (t *Trie) UpdateAccount(key []byte, account *types.StateAccount) {
	if err := t.TryUpdateAccount(key, account); err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
}

// TryUpdateAccount will abstract the write of an account to the
// secure trie.
func (t *Trie) TryUpdateAccount(key []byte, acc *types.StateAccount) error {
	value, flag := acc.MarshalFields()
	return t.impl.TryUpdate(bytesToHash(key), flag, value)
}

// NOTE: value is restricted to length of bytes32.
// we override the underlying itrie's TryUpdate method
func (t *Trie) TryUpdate(key, value []byte) error {
	return t.impl.TryUpdate(bytesToHash(key), 1, []itypes.Byte32{*itypes.NewByte32FromBytes(value)})
}

func (t *Trie) TryDelete(key []byte) error {
	return t.impl.TryDelete(bytesToHash(key))
}

// Delete removes any existing value for key from the trie.
func (t *Trie) Delete(key []byte) {
	if err := t.impl.TryDelete(bytesToHash(key)); err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
}

func (t *Trie) TryGetNode(path []byte) ([]byte, int, error) {
	panic("not implemented")
}

// Commit writes all nodes and the secure hash pre-images to the trie's database.
// Nodes are stored with their sha3 hash as the key.
//
// Committing flushes nodes from memory. Subsequent Get calls will load nodes
// from the database.
func (t *Trie) Commit(LeafCallback) (common.Hash, int, error) {
	// in current implmentation, every update of trie already writes into database
	// so Commmit does nothing
	return t.Hash(), 0, nil
}

// Hash returns the root hash of SecureBinaryTrie. It does not write to the
// database and can be used even if the trie doesn't have one.
func (t *Trie) Hash() common.Hash {
	if t.impl == nil {
		return emptyRoot
	}
	var hash common.Hash
	hash.SetBytes(t.impl.Root().Bytes())
	return hash
}

// NodeIterator returns an iterator that returns nodes of the underlying trie. Iteration
// starts at the key after the given start key.
func (t *Trie) NodeIterator(start []byte) trie.NodeIterator {
	/// FIXME
	panic("not implemented")
}
