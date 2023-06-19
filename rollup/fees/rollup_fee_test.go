package fees

import (
	"bytes"
	"math/big"
	"testing"

	"encoding/json"

	"github.com/scroll-tech/go-ethereum/common/hexutil"
	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func TestCalculateEncodedL1DataFee(t *testing.T) {
	l1BaseFee := new(big.Int).SetUint64(15000000)

	data := []byte{0, 10, 1, 0}
	overhead := new(big.Int).SetUint64(100)
	scalar := new(big.Int).SetUint64(10)

	expected := new(big.Int).SetUint64(30) // 30.8
	actual := calculateEncodedL1DataFee(data, overhead, l1BaseFee, scalar)
	assert.Equal(t, expected, actual)
}

const example_tx1 = `
	{
		"type": 0,
		"nonce": 4,
		"txHash": "0x8da3fedb103b6da8ccc2514094336d1a76df166238f4d8e8558fbe54cce2516a",
		"gas": 53854,
		"gasPrice": "0x3b9aca00",
		"from": "0x1c5a77d9fa7ef466951b2f01f724bca3a5820b63",
		"to": "0x03f8133dd5ed58838b20af1296f62f44e69baa48",
		"chainId": "0xcf55",
		"value": "0x0",
		"data": "0xa9059cbb000000000000000000000000c0c4c8baea3f6acb49b6e1fb9e2adeceeacb0ca200000000000000000000000000000000000000000000000000000000000003e8",
		"isCreate": false
	}
`
const example_tx2 = `
	{
		"type": 0,
		"nonce": 20,
		"txHash": "0xfef778b40acae6c4f00205f3dafae2af1dff90d402c19b090c4b12cad08e7461",
		"gas": 23730,
		"gasPrice": "0x3b9aca00",
		"from": "0x1c5a77d9fa7ef466951b2f01f724bca3a5820b63",
		"to": "0x58a2239aa5412f78d8f675c4d8ad5102a3fa5837",
		"chainId": "0xcf55",
		"value": "0x0",
		"data": "0xb0f2b72a000000000000000000000000000000000000000000000000000000000000000a",
		"isCreate": false
	}
`

func reverseDataToMsg(txdata *types.TransactionData) types.Message {
	databytes, err := hexutil.Decode(txdata.Data)
	if err != nil {
		panic(err)
	}
	return types.NewMessage(txdata.From, txdata.To, txdata.Nonce, (*big.Int)(txdata.Value), txdata.Gas,
		(*big.Int)(txdata.GasPrice), (*big.Int)(txdata.GasPrice), (*big.Int)(txdata.GasPrice), databytes, nil, false)
}

func TestCalculateL1DataSize(t *testing.T) {
	txdata := new(types.TransactionData)
	assert.NoError(t, json.Unmarshal([]byte(example_tx1), txdata), "parse json fail")

	t.Log(txdata)

	msg := reverseDataToMsg(txdata)

	chainID := big.NewInt(53077) //0xcf55
	var eip1559baseFee *big.Int
	//eip1559baseFee = new(big.Int).SetUint64(15000000)
	signer := types.NewLondonSigner(chainID)
	t.Log(msg)

	unsigned := asUnsignedTx(msg, eip1559baseFee, chainID)

	tx, err := unsigned.WithSignature(signer, append(bytes.Repeat([]byte{0xff}, crypto.SignatureLength-1), 0x01))
	assert.NoError(t, err, "build dummy tx fail")
	raw, err := rlpEncode(tx)
	t.Log(raw)
	assert.NoError(t, err, "rlp fail")

	assert.Equal(t, 173, len(raw))

	basefee := big.NewInt(0x64)
	overhead := big.NewInt(0x17d4)
	scalar := big.NewInt(0x4a42fc80)

	l1DataFee := calculateEncodedL1DataFee(raw, overhead, basefee, scalar)
	assert.Equal(t, big.NewInt(0xfffe8), l1DataFee)
}
