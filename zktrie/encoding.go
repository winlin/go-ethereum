package zktrie

import itypes "github.com/scroll-tech/zktrie/types"

// binary encoding
type BinaryPath []bool

func bytesToPath(b []byte) BinaryPath {
	panic("not implemented")
}

func bytesToHash(b []byte) *itypes.Hash {
	var h itypes.Hash
	copy(h[:], b)
	return &h
}

func hashToBytes(hash *itypes.Hash) []byte {
	return hash[:]
}
