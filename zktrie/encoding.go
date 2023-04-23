package zktrie

import itypes "github.com/scroll-tech/zktrie/types"

type BinaryPath struct {
	d    []byte
	size int
}

func NewBinaryPathFromKeyBytes(b []byte) *BinaryPath {
	d := make([]byte, 8*len(b))
	copy(d, b)
	return &BinaryPath{
		size: len(b) * 8,
		d:    d,
	}
}

func (bp *BinaryPath) Size() int {
	return bp.size
}

func (bp *BinaryPath) Pos(i int) bool {
	return (bp.d[i/8] & (1 << (i % 8))) != 0
}

func (bp *BinaryPath) ToKeyBytes() []byte {
	if bp.size%8 != 0 {
		panic("can't convert binary key whose size is not multiple of 8")
	}
	d := make([]byte, bp.size)
	copy(d, bp.d)
	return d
}

func bytesToHash(b []byte) *itypes.Hash {
	var h itypes.Hash
	copy(h[:], b)
	return &h
}
