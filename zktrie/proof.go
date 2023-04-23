package zktrie

import (
	"bytes"
	"fmt"

	itrie "github.com/scroll-tech/zktrie/trie"
	itypes "github.com/scroll-tech/zktrie/types"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/ethdb"
)

// Prove constructs a merkle proof for key. The result contains all encoded nodes
// on the path to the value at key. The value itself is also included in the last
// node and can be retrieved by verifying the proof.
//
// If the trie does not contain a value for key, the returned proof contains all
// nodes of the longest existing prefix of the key (at least the root node), ending
// with the node that proves the absence of the key.
func (t *SecureTrie) Prove(key []byte, fromLevel uint, proofDb ethdb.KeyValueWriter) error {
	// omit sibling, which is not required for proving only
	_, err := t.ProveWithDeletion(key, fromLevel, proofDb)
	return err
}

func (t *SecureTrie) ProveWithDeletion(key []byte, fromLevel uint, proofDb ethdb.KeyValueWriter) (sibling []byte, err error) {
	// standardize the key format, which is the same as trie interface
	key = itypes.ReverseByteOrder(key)
	err = t.trie.ProveWithDeletion(key, fromLevel,
		func(n *itrie.Node) error {
			nodeHash, err := n.NodeHash()
			if err != nil {
				return err
			}

			if n.Type == itrie.NodeTypeLeaf {
				preImage := t.GetKey(n.NodeKey.Bytes())
				if len(preImage) > 0 {
					n.KeyPreimage = &itypes.Byte32{}
					copy(n.KeyPreimage[:], preImage)
					//return fmt.Errorf("key preimage not found for [%x] ref %x", n.NodeKey.Bytes(), k.Bytes())
				}
			}
			return proofDb.Put(nodeHash[:], n.Value())
		},
		func(_ *itrie.Node, n *itrie.Node) {
			// the sibling for each leaf should be unique except for EmptyNode
			if n != nil && n.Type != itrie.NodeTypeEmpty {
				sibling = n.Value()
			}
		},
	)
	return
}

// Prove constructs a merkle proof for key. The result contains all encoded nodes
// on the path to the value at key. The value itself is also included in the last
// node and can be retrieved by verifying the proof.
//
// If the trie does not contain a value for key, the returned proof contains all
// nodes of the longest existing prefix of the key (at least the root node), ending
// with the node that proves the absence of the key.
func (t *Trie) Prove(key []byte, fromLevel uint, proofDb ethdb.KeyValueWriter) error {
	// omit sibling, which is not required for proving only
	_, err := t.ProveWithDeletion(key, fromLevel, proofDb)
	return err
}

// ProveWithDeletion is the implement of Prove, it also return possible sibling node
// (if there is, i.e. the node of key exist and is not the only node in trie)
// so witness generator can predict the final state root after deletion of this key
// the returned sibling node has no key along with it for witness generator must decode
// the node for its purpose
func (t *Trie) ProveWithDeletion(key []byte, fromLevel uint, proofDb ethdb.KeyValueWriter) (sibling []byte, err error) {
	// standardize the key format, which is the same as trie interface
	key = itypes.ReverseByteOrder(key)
	err = t.tr.ProveWithDeletion(key, fromLevel,
		func(n *itrie.Node) error {
			nodeHash, err := n.NodeHash()
			if err != nil {
				return err
			}
			return proofDb.Put(nodeHash[:], n.Value())
		},
		func(_ *itrie.Node, n *itrie.Node) {
			// the sibling for each leaf should be unique except for EmptyNode
			if n != nil && n.Type != itrie.NodeTypeEmpty {
				sibling = n.Value()
			}
		},
	)
	return
}

// VerifyProof checks merkle proofs. The given proof must contain the value for
// key in a trie with the given root hash. VerifyProof returns an error if the
// proof contains invalid trie nodes or the wrong value.
func VerifyProof(rootHash common.Hash, key []byte, proofDb ethdb.KeyValueReader) (value []byte, err error) {
	path := NewBinaryPathFromKeyBytes(key)
	wantHash := zktNodeHash(rootHash)
	for i := 0; i < path.Size(); i++ {
		buf, _ := proofDb.Get(wantHash[:])
		if buf == nil {
			return nil, fmt.Errorf("proof node %d (hash %064x) missing", i, wantHash)
		}
		n, err := itrie.NewNodeFromBytes(buf)
		if err != nil {
			return nil, fmt.Errorf("bad proof node %d: %v", i, err)
		}
		switch n.Type {
		case itrie.NodeTypeEmpty:
			return n.Data(), nil
		case itrie.NodeTypeLeaf:
			if bytes.Equal(key, n.NodeKey[:]) {
				return n.Data(), nil
			}
			// We found a leaf whose entry didn't match hIndex
			return nil, nil
		case itrie.NodeTypeParent:
			if path.Pos(i) {
				wantHash = n.ChildR
			} else {
				wantHash = n.ChildL
			}
		default:
			return nil, itrie.ErrInvalidNodeFound
		}
	}
	return nil, itrie.ErrKeyNotFound
}

func VerifyRangeProof(rootHash common.Hash, firstKey []byte, lastKey []byte, keys [][]byte, values [][]byte, proof ethdb.KeyValueReader) (bool, error) {
	panic("not implemented")
}
