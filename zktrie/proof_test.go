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
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	zkt "github.com/scroll-tech/zktrie/types"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/ethdb/memorydb"
)

func init() {
	mrand.Seed(time.Now().Unix())
}

// makeProvers creates Merkle trie provers based on different implementations to
// test all variations.
func makeTrieProvers(tr *Trie) []func(key []byte) *memorydb.Database {
	var provers []func(key []byte) *memorydb.Database

	// Create a direct trie based Merkle prover
	provers = append(provers, func(key []byte) *memorydb.Database {
		proof := memorydb.New()
		err := tr.Prove(key, 0, proof)
		if err != nil {
			panic(err)
		}

		return proof
	})
	return provers
}

func makeSecureTrieProvers(tr *SecureTrie) []func(key []byte) *memorydb.Database {
	var provers []func(key []byte) *memorydb.Database

	// Create a direct trie based Merkle prover
	provers = append(provers, func(key []byte) *memorydb.Database {
		proof := memorydb.New()
		err := tr.Prove(key, 0, proof)
		if err != nil {
			panic(err)
		}

		return proof
	})
	return provers
}

func verifyValue(proveVal []byte, vPreimage []byte) bool {
	return bytes.Equal(proveVal, vPreimage)
}

func TestTrieOneElementProof(t *testing.T) {
	tr, _ := New(common.Hash{}, NewDatabase(memorydb.New()))
	key := zkt.NewByte32FromBytesPaddingZero(append(bytes.Repeat([]byte("k"), 10), bytes.Repeat([]byte("l"), 21)...)).Bytes()
	err := tr.TryUpdate(key, bytes.Repeat([]byte("v"), 32))
	assert.Nil(t, err)
	for i, prover := range makeTrieProvers(tr) {
		proof := prover(key)
		if proof == nil {
			t.Fatalf("prover %d: nil proof", i)
		}
		if proof.Len() != 2 {
			t.Errorf("prover %d: proof should have 1+1 element (including the magic kv)", i)
		}
		val, err := VerifyProof(tr.Hash(), key, proof)
		if err != nil {
			t.Fatalf("prover %d: failed to verify proof: %v\nraw proof: %x", i, err, proof)
		}
		if !verifyValue(val, bytes.Repeat([]byte("v"), 32)) {
			t.Fatalf("prover %d: verified value mismatch: want 'v' get %x", i, val)
		}
	}
}

func TestSecureTrieOneElementProof(t *testing.T) {
	tr, _ := NewSecure(common.Hash{}, NewDatabase(memorydb.New()))
	key := zkt.NewByte32FromBytesPaddingZero(append(bytes.Repeat([]byte("k"), 10), bytes.Repeat([]byte("l"), 21)...)).Bytes()
	err := tr.TryUpdate(key, bytes.Repeat([]byte("v"), 32))
	assert.Nil(t, err)
	for i, prover := range makeSecureTrieProvers(tr) {
		secureKey := toSecureKey(key)
		proof := prover(secureKey)
		if proof == nil {
			t.Fatalf("prover %d: nil proof", i)
		}
		if proof.Len() != 2 {
			t.Errorf("prover %d: proof should have 1+1 element (including the magic kv)", i)
		}
		val, err := VerifyProof(tr.Hash(), secureKey, proof)
		if err != nil {
			t.Fatalf("prover %d: failed to verify proof: %v\nraw proof: %x", i, err, proof)
		}
		if !verifyValue(val, bytes.Repeat([]byte("v"), 32)) {
			t.Fatalf("prover %d: verified value mismatch: want 'v' get %x", i, val)
		}
	}
}

func TestTrieProof(t *testing.T) {
	tr, vals := randomTrie(t, 500)
	root := tr.Hash()
	for i, prover := range makeTrieProvers(tr) {
		for _, kv := range vals {
			proof := prover(kv.k)
			if proof == nil {
				t.Fatalf("prover %d: missing key %x while constructing proof", i, kv.k)
			}
			val, err := VerifyProof(common.BytesToHash(root.Bytes()), kv.k, proof)
			if err != nil {
				t.Fatalf("prover %d: failed to verify proof for key %x: %v\nraw proof: %x\n", i, kv.k, err, proof)
			}
			if !verifyValue(val, zkt.NewByte32FromBytesPaddingZero(kv.v)[:]) {
				t.Fatalf("prover %d: verified value mismatch for key %x, want %x, get %x", i, kv.k, kv.v, val)
			}
		}
	}
}

func TestSecureTrieProof(t *testing.T) {
	tr, vals := randomSecureTrie(t, 500)
	root := tr.Hash()
	for i, prover := range makeSecureTrieProvers(tr) {
		for _, kv := range vals {
			secureKey := toSecureKey(kv.k)
			proof := prover(secureKey)
			if proof == nil {
				t.Fatalf("prover %d: missing key %x while constructing proof", i, secureKey)
			}
			val, err := VerifyProof(common.BytesToHash(root.Bytes()), secureKey, proof)
			if err != nil {
				t.Fatalf("prover %d: failed to verify proof for key %x: %v\nraw proof: %x\n", i, secureKey, err, proof)
			}
			if !verifyValue(val, zkt.NewByte32FromBytesPaddingZero(kv.v)[:]) {
				t.Fatalf("prover %d: verified value mismatch for key %x, want %x, get %x", i, secureKey, kv.v, val)
			}
		}
	}
}

func TestTrieBadProof(t *testing.T) {
	tr, vals := randomTrie(t, 500)
	for i, prover := range makeTrieProvers(tr) {
		for _, kv := range vals {
			proof := prover(kv.k)
			if proof == nil {
				t.Fatalf("prover %d: nil proof", i)
			}
			it := proof.NewIterator(nil, nil)
			for i, d := 0, mrand.Intn(proof.Len()-1); i <= d; i++ {
				it.Next()
			}

			// Need to randomly mutate two keys, as magic kv in Proof is not used in verifyProof
			for i := 0; i <= 2; i++ {
				key := it.Key()
				proof.Delete(key)
				it.Next()
			}
			it.Release()

			if _, err := VerifyProof(tr.Hash(), kv.k, proof); err == nil {
				t.Fatalf("prover %d: expected proof to fail for key %x", i, kv.k)
			}
		}
	}
}

func TestSecureTrieBadProof(t *testing.T) {
	tr, vals := randomSecureTrie(t, 500)
	for i, prover := range makeSecureTrieProvers(tr) {
		for _, kv := range vals {
			secureKey := toSecureKey(kv.k)
			proof := prover(secureKey)
			if proof == nil {
				t.Fatalf("prover %d: nil proof", i)
			}
			it := proof.NewIterator(nil, nil)
			for i, d := 0, mrand.Intn(proof.Len()-1); i <= d; i++ {
				it.Next()
			}

			// Need to randomly mutate two keys, as magic kv in Proof is not used in verifyProof
			for i := 0; i <= 2; i++ {
				key := it.Key()
				proof.Delete(key)
				it.Next()
			}
			it.Release()

			if _, err := VerifyProof(tr.Hash(), secureKey, proof); err == nil {
				t.Fatalf("prover %d: expected proof to fail for key %x", i, secureKey)
			}
		}
	}
}

// Tests that missing keys can also be proven. The test explicitly uses a single
// entry trie and checks for missing keys both before and after the single entry.
func TestTrieMissingKeyProof(t *testing.T) {
	tr, _ := New(common.Hash{}, NewDatabase(memorydb.New()))
	key := zkt.NewByte32FromBytesPaddingZero(append(bytes.Repeat([]byte("k"), 10), bytes.Repeat([]byte("l"), 21)...)).Bytes()
	err := tr.TryUpdate(key, bytes.Repeat([]byte("v"), 32))
	assert.Nil(t, err)

	prover := makeTrieProvers(tr)[0]

	for i, key := range []string{"a", "j", "l", "z"} {
		keyBytes := bytes.Repeat([]byte(key), 32)
		proof := prover(keyBytes)

		if proof.Len() != 2 {
			t.Errorf("test %d: proof should have 2 element (with magic kv)", i)
		}
		val, err := VerifyProof(tr.Hash(), keyBytes, proof)
		if err != nil {
			t.Fatalf("test %d: failed to verify proof: %v\nraw proof: %x", i, err, proof)
		}
		if val != nil {
			t.Fatalf("test %d: verified value mismatch: have %x, want nil", i, val)
		}
	}
}

func TestSecureTrieMissingKeyProof(t *testing.T) {
	tr, _ := NewSecure(common.Hash{}, NewDatabase(memorydb.New()))
	key := zkt.NewByte32FromBytesPaddingZero(append(bytes.Repeat([]byte("k"), 10), bytes.Repeat([]byte("l"), 21)...)).Bytes()
	err := tr.TryUpdate(key, bytes.Repeat([]byte("v"), 32))
	assert.Nil(t, err)

	prover := makeSecureTrieProvers(tr)[0]

	for i, key := range []string{"a", "j", "l", "z"} {
		keyBytes := bytes.Repeat([]byte(key), 32)
		secureKey := toSecureKey(keyBytes)
		proof := prover(secureKey)

		if proof.Len() != 2 {
			t.Errorf("test %d: proof should have 2 element (with magic kv)", i)
		}
		val, err := VerifyProof(tr.Hash(), secureKey, proof)
		if err != nil {
			t.Fatalf("test %d: failed to verify proof: %v\nraw proof: %x", i, err, proof)
		}
		if val != nil {
			t.Fatalf("test %d: verified value mismatch: have %x, want nil", i, val)
		}
	}
}

// Tests that new "proof with deletion" feature
func TestTrieProofWithDeletion(t *testing.T) {
	tr, _ := New(common.Hash{}, NewDatabase((memorydb.New())))
	key1 := zkt.NewByte32FromBytesPaddingZero(append(bytes.Repeat([]byte("k"), 10), bytes.Repeat([]byte("l"), 21)...)).Bytes()
	key2 := zkt.NewByte32FromBytesPaddingZero(append(bytes.Repeat([]byte("m"), 10), bytes.Repeat([]byte("n"), 21)...)).Bytes()

	err := tr.TryUpdate(key1, bytes.Repeat([]byte("v"), 32))
	assert.NoError(t, err)
	err = tr.TryUpdate(key2, bytes.Repeat([]byte("v"), 32))
	assert.NoError(t, err)

	proof := memorydb.New()
	assert.NoError(t, err)

	sibling1, err := tr.ProveWithDeletion(key1, 0, proof)
	assert.NoError(t, err)
	nd, err := tr.TryGet(key2)
	assert.NoError(t, err)
	l := len(sibling1)
	// a hacking to grep the value part directly from the encoded leaf node,
	// notice the sibling of key1 is just the leaf of key2
	assert.Equal(t, sibling1[l-33:l-1], nd)

	notKey := zkt.NewByte32FromBytesPaddingZero(bytes.Repeat([]byte{'x'}, 31)).Bytes()
	sibling2, err := tr.ProveWithDeletion(notKey, 0, proof)
	assert.NoError(t, err)
	assert.Nil(t, sibling2)
}

func TestSecureTrieProofWithDeletion(t *testing.T) {
	tr, _ := NewSecure(common.Hash{}, NewDatabase((memorydb.New())))
	key1 := zkt.NewByte32FromBytesPaddingZero(append(bytes.Repeat([]byte("k"), 10), bytes.Repeat([]byte("l"), 21)...)).Bytes()
	key2 := zkt.NewByte32FromBytesPaddingZero(append(bytes.Repeat([]byte("m"), 10), bytes.Repeat([]byte("n"), 21)...)).Bytes()
	secureKey1 := toSecureKey(key1)

	err := tr.TryUpdate(key1, bytes.Repeat([]byte("v"), 32))
	assert.NoError(t, err)
	err = tr.TryUpdate(key2, bytes.Repeat([]byte("v"), 32))
	assert.NoError(t, err)

	proof := memorydb.New()
	assert.NoError(t, err)

	sibling1, err := tr.ProveWithDeletion(secureKey1, 0, proof)
	assert.NoError(t, err)
	nd, err := tr.TryGet(key2)
	assert.NoError(t, err)
	l := len(sibling1)
	// a hacking to grep the value part directly from the encoded leaf node,
	// notice the sibling of key1 is just the leaf of key2
	assert.Equal(t, sibling1[l-33:l-1], nd)

	notKey := zkt.NewByte32FromBytesPaddingZero(bytes.Repeat([]byte{'x'}, 31)).Bytes()
	sibling2, err := tr.ProveWithDeletion(notKey, 0, proof)
	assert.NoError(t, err)
	assert.Nil(t, sibling2)
}

func randBytes(n int) []byte {
	r := make([]byte, n)
	crand.Read(r)
	return r
}

func randomTrie(t *testing.T, n int) (*Trie, map[string]*kv) {
	tr, err := New(common.Hash{}, NewDatabase((memorydb.New())))
	if err != nil {
		panic(err)
	}
	vals := make(map[string]*kv)
	for i := byte(0); i < 100; i++ {

		value := &kv{zkt.NewByte32FromBytesPaddingZero(bytes.Repeat([]byte{i}, 31)).Bytes(), bytes.Repeat([]byte{i}, 32), false}
		value2 := &kv{zkt.NewByte32FromBytesPaddingZero(bytes.Repeat([]byte{i + 10}, 31)).Bytes(), bytes.Repeat([]byte{i + 5}, 32), false}

		err = tr.TryUpdate(value.k, value.v)
		assert.Nil(t, err)
		err = tr.TryUpdate(value2.k, value2.v)
		assert.Nil(t, err)
		vals[string(value.k)] = value
		vals[string(value2.k)] = value2
	}
	for i := 0; i < n; i++ {
		value := &kv{zkt.NewByte32FromBytesPaddingZero(randBytes(31)).Bytes(), randBytes(32), false}
		err = tr.TryUpdate(value.k, value.v)
		assert.Nil(t, err)
		vals[string(value.k)] = value
	}

	return tr, vals
}

func randomSecureTrie(t *testing.T, n int) (*SecureTrie, map[string]*kv) {
	tr, err := NewSecure(common.Hash{}, NewDatabase((memorydb.New())))
	if err != nil {
		panic(err)
	}
	vals := make(map[string]*kv)
	for i := byte(0); i < 100; i++ {

		value := &kv{zkt.NewByte32FromBytesPaddingZero(bytes.Repeat([]byte{i}, 31)).Bytes(), bytes.Repeat([]byte{i}, 32), false}
		value2 := &kv{zkt.NewByte32FromBytesPaddingZero(bytes.Repeat([]byte{i + 10}, 31)).Bytes(), bytes.Repeat([]byte{i + 5}, 32), false}

		err = tr.TryUpdate(value.k, value.v)
		assert.Nil(t, err)
		err = tr.TryUpdate(value2.k, value2.v)
		assert.Nil(t, err)
		vals[string(value.k)] = value
		vals[string(value2.k)] = value2
	}
	for i := 0; i < n; i++ {
		value := &kv{zkt.NewByte32FromBytesPaddingZero(randBytes(31)).Bytes(), randBytes(32), false}
		err = tr.TryUpdate(value.k, value.v)
		assert.Nil(t, err)
		vals[string(value.k)] = value
	}

	return tr, vals
}

type entrySlice []*kv

func (p entrySlice) Len() int           { return len(p) }
func (p entrySlice) Less(i, j int) bool { return bytes.Compare(p[i].k, p[j].k) < 0 }
func (p entrySlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func TestSimpleProofValidRange(t *testing.T) {
	trie, kvs := nonRandomTrie(5)
	var entries entrySlice
	for _, kv := range kvs {
		entries = append(entries, kv)
		fmt.Printf("%v\n", kv)
	}
	sort.Sort(entries)

	proof := memorydb.New()
	if err := trie.Prove(entries[1].k, 0, proof); err != nil {
		t.Fatalf("Failed to prove the first node %v", err)
	}
	if err := trie.Prove(entries[3].k, 0, proof); err != nil {
		t.Fatalf("Failed to prove the last node %v", err)
	}

	var keys [][]byte
	var vals [][]byte
	for i := 1; i <= 3; i++ {
		keys = append(keys, entries[i].k)
		vals = append(vals, entries[i].v)
	}
	_, err := VerifyRangeProof(trie.Hash(), "storage", keys[0], keys[len(keys)-1], keys, vals, proof)
	if err != nil {
		t.Fatalf("Verification of range proof failed!\n%v\n", err)
	}
}

func nonRandomTrie(n int) (*Trie, map[string]*kv) {
	trie, err := New(common.Hash{}, NewDatabase((memorydb.New())))
	if err != nil {
		panic(err)
	}
	vals := make(map[string]*kv)
	max := uint64(0xffffffffffffffff)
	for i := uint64(0); i < uint64(n); i++ {
		value := make([]byte, 32)
		key := make([]byte, 32)
		binary.LittleEndian.PutUint64(key, i)
		binary.LittleEndian.PutUint64(value, i-max)
		//value := &kv{common.LeftPadBytes([]byte{i}, 32), []byte{i}, false}
		elem := &kv{key, value, false}
		trie.Update(elem.k, elem.v)
		vals[string(elem.k)] = elem
	}
	return trie, vals
}
