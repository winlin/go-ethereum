package zktrie

import (
	itypes "github.com/scroll-tech/zktrie/types"

	"github.com/scroll-tech/go-ethereum/common/hexutil"
)

type BinaryPath struct {
	d    []byte
	size int
}

func keyBytesToHex(b []byte) string {
	return hexutil.Encode(b)
}

func NewBinaryPathFromKeyBytes(b []byte) *BinaryPath {
	d := make([]byte, len(b))
	copy(d, b)
	return &BinaryPath{
		size: len(b) * 8,
		d:    d,
	}
}

func (bp *BinaryPath) Size() int {
	return bp.size
}

func (bp *BinaryPath) Pos(i int) int8 {
	if (bp.d[i/8] & (1 << (7 - (i % 8)))) != 0 {
		return 1
	} else {
		return 0
	}
}

func (bp *BinaryPath) ToKeyBytes() []byte {
	if bp.size%8 != 0 {
		panic("can't convert binary key whose size is not multiple of 8")
	}
	d := make([]byte, len(bp.d))
	copy(d, bp.d)
	return d
}

func reverseBitInPlace(b []byte) {
	var v [8]uint8
	for i := 0; i < len(b); i++ {
		for j := 0; j < 8; j++ {
			v[j] = (b[i] >> j) & 1
		}
		var tmp uint8 = 0
		for j := 0; j < 8; j++ {
			tmp |= v[8-j-1] << j
		}
		b[i] = tmp
	}

}
func KeybytesToHashKey(b []byte) *itypes.Hash {
	var h itypes.Hash
	copy(h[:], b)
	reverseBitInPlace(h[:])
	return &h
}

func HashKeyToKeybytes(h *itypes.Hash) []byte {
	b := make([]byte, itypes.HashByteLen)
	copy(b, h[:])
	reverseBitInPlace(b)
	return b
}
