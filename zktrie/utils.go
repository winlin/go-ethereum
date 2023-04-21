package zktrie

import (
	itypes "github.com/scroll-tech/zktrie/types"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/crypto/poseidon"
)

func init() {
	itypes.InitHashScheme(poseidon.HashFixed)
}

func zktNodeHash(node common.Hash) *itypes.Hash {
	byte32 := itypes.NewByte32FromBytes(node.Bytes())
	return itypes.NewHashFromBytes(byte32.Bytes())
}
