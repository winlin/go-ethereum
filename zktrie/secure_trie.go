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

	itypes "github.com/scroll-tech/zktrie/types"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/log"
)

var magicHash []byte = []byte("THIS IS THE MAGIC INDEX FOR ZKTRIE")

// wrap itrie for trie interface
type SecureTrie struct {
	trie Trie
}

// New creates a trie
// New bypasses all the buffer mechanism in *Database, it directly uses the
// underlying diskdb
func NewSecure(root common.Hash, db *Database) (*SecureTrie, error) {
	if db == nil {
		panic("zktrie.NewSecure called without a database")
	}
	t, err := New(root, db)
	if err != nil {
		return nil, err
	}
	return &SecureTrie{trie: *t}, nil
}

func (t *SecureTrie) hashKey(key []byte) []byte {
	i, err := itypes.ToSecureKey(key)
	if err != nil {
		log.Error(fmt.Sprintf("unhandled secure trie error: %v", err))
	}
	hash := itypes.NewHashFromBigInt(i)
	return hashToBytes(hash)
}

// Get returns the value for key stored in the trie.
// The value bytes must not be modified by the caller.
func (t *SecureTrie) Get(key []byte) []byte {
	res, err := t.trie.TryGet(t.hashKey(key))
	if err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
	return res
}

func (t *SecureTrie) TryGet(key []byte) ([]byte, error) {
	return t.trie.TryGet(t.hashKey(key))
}

// TryUpdateAccount will abstract the write of an account to the
// secure trie.
func (t *SecureTrie) TryUpdateAccount(key []byte, acc *types.StateAccount) error {
	return t.trie.TryUpdateAccount(t.hashKey(key), acc)
}

// Update associates key with value in the trie. Subsequent calls to
// Get will return value. If value has length zero, any existing value
// is deleted from the trie and calls to Get will return nil.
//
// The value bytes must not be modified by the caller while they are
// stored in the trie.
func (t *SecureTrie) Update(key, value []byte) {
	if err := t.TryUpdate(key, value); err != nil {
		log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	}
}

// NOTE: value is restricted to length of bytes32.
// we override the underlying itrie's TryUpdate method
func (t *SecureTrie) TryUpdate(key, value []byte) error {
	return t.trie.TryUpdate(t.hashKey(key), value)
}

// Delete removes any existing value for key from the trie.
func (t *SecureTrie) Delete(key []byte) {
	if err := t.trie.TryDelete(t.hashKey(key)); err != nil {
		log.Error(fmt.Sprintf("Unhandled secure trie error: %v", err))
	}
}

func (t *SecureTrie) TryDelete(key []byte) error {
	return t.trie.TryDelete(t.hashKey(key))
}

// GetKey returns the preimage of a hashed key that was
// previously used to store a value.
func (t *SecureTrie) GetKey(kHashBytes []byte) []byte {
	panic("not implemented")
	// TODO: use a kv cache in memory, need preimage
	//k, err := itypes.NewBigIntFromHashBytes(kHashBytes)
	//if err != nil {
	//	log.Error(fmt.Sprintf("Unhandled trie error: %v", err))
	//}
	//if t.db.preimages != nil {
	//	return t.db.preimages.preimage(common.BytesToHash(k.Bytes()))
	//}
	//return nil
}

// Commit writes all nodes and the secure hash pre-images to the trie's database.
// Nodes are stored with their sha3 hash as the key.
//
// Committing flushes nodes from memory. Subsequent Get calls will load nodes
// from the database.
func (t *SecureTrie) Commit(LeafCallback) (common.Hash, int, error) {
	// in current implmentation, every update of trie already writes into database
	// so Commmit does nothing
	return t.Hash(), 0, nil
}

// Hash returns the root hash of SecureBinaryTrie. It does not write to the
// database and can be used even if the trie doesn't have one.
func (t *SecureTrie) Hash() common.Hash {
	return t.trie.Hash()
}

// Copy returns a copy of SecureBinaryTrie.
func (t *SecureTrie) Copy() *SecureTrie {
	cpy := *t
	return &cpy
}

// NodeIterator returns an iterator that returns nodes of the underlying trie. Iteration
// starts at the key after the given start key.
func (t *SecureTrie) NodeIterator(start []byte) NodeIterator {
	/// FIXME
	panic("not implemented")
}

func (t *SecureTrie) TryGetNode(path []byte) ([]byte, int, error) {
	return t.trie.TryGetNode(path)
}

// hashKey returns the hash of key as an ephemeral buffer.
// The caller must not hold onto the return value because it will become
// invalid on the next call to hashKey or secKey.
/*func (t *Trie) hashKey(key []byte) []byte {
	if len(key) != 32 {
		panic("non byte32 input to hashKey")
	}
	low16 := new(big.Int).SetBytes(key[:16])
	high16 := new(big.Int).SetBytes(key[16:])
	hash, err := poseidon.Hash([]*big.Int{low16, high16})
	if err != nil {
		panic(err)
	}
	return hash.Bytes()
}
*/
