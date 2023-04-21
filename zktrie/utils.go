package zktrie

import (
	"fmt"

	itypes "github.com/scroll-tech/zktrie/types"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/crypto/poseidon"
)

func init() {
	itypes.InitHashScheme(poseidon.HashFixed)
}

func sanityCheckByte32Key(b []byte) {
	if len(b) != 32 && len(b) != 20 {
		panic(fmt.Errorf("do not support length except for 120bit and 256bit now. data: %v len: %v", b, len(b)))
	}
}

func zktNodeHash(node common.Hash) *itypes.Hash {
	byte32 := itypes.NewByte32FromBytes(node.Bytes())
	return itypes.NewHashFromBytes(byte32.Bytes())
}
