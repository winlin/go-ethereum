package zkproof

import itypes "github.com/scroll-tech/zktrie/types"

func toProveKey(b []byte) []byte {
	k, _ := itypes.ToSecureKey(b)
	return itypes.NewHashFromBigInt(k)[:]
}
