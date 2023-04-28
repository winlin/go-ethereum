package zktrie

import (
	itrie "github.com/scroll-tech/zktrie/trie"
	itypes "github.com/scroll-tech/zktrie/types"

	"github.com/scroll-tech/go-ethereum/common/hexutil"
)

func keyBytesToHex(b []byte) string {
	return hexutil.Encode(b)
}

func keybytesToBinary(b []byte) []byte {
	d := make([]byte, 0, 8*len(b))
	for i := 0; i < len(b); i++ {
		for j := 0; j < 8; j++ {
			if b[i]&(1<<(7-j)) == 0 {
				d = append(d, 0)
			} else {
				d = append(d, 1)
			}
		}
	}
	return d
}

func BinaryToKeybytes(b []byte) []byte {
	if len(b)%8 != 0 {
		panic("can't convert binary key whose size is not multiple of 8")
	}
	d := make([]byte, len(b)/8)
	for i := 0; i < len(b)/8; i++ {
		d[i] = 0
		for j := 0; j < 8; j++ {
			d[i] |= b[i*8+j] << (7 - j)
		}
	}
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

func KeybytesToHashKeyAndCheck(b []byte) (*itypes.Hash, error) {
	var h itypes.Hash
	copy(h[:], b)
	reverseBitInPlace(h[:])
	if !itypes.CheckBigIntInField(h.BigInt()) {
		return nil, itrie.ErrInvalidField
	}
	return &h, nil
}

func HashKeyToKeybytes(h *itypes.Hash) []byte {
	b := make([]byte, itypes.HashByteLen)
	copy(b, h[:])
	reverseBitInPlace(b)
	return b
}

func HashKeyToBinary(h *itypes.Hash) []byte {
	kb := HashKeyToKeybytes(h)
	return keybytesToBinary(kb)
}
