package zkproof

import (
	"fmt"

	itypes "github.com/scroll-tech/zktrie/types"

	"github.com/scroll-tech/go-ethereum/log"
	"github.com/scroll-tech/go-ethereum/zktrie"
)

func ToProveKey(b []byte) []byte {
	if k, err := itypes.ToSecureKey(b); err != nil {
		log.Error(fmt.Sprintf("unhandled error: %v", err))
		return nil
	} else {
		return zktrie.HashKeyToKeybytes(itypes.NewHashFromBigInt(k))
	}
}
