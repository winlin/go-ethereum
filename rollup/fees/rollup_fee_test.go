package fees

import (
	"bytes"
	"math/big"
	"testing"

	"encoding/json"

	"github.com/scroll-tech/go-ethereum/common/hexutil"
	"github.com/scroll-tech/go-ethereum/core/types"
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
		"isCreate": false,
		"v": "0x19ecd",
		"r": "0xaaa87d285f44e2683266d83116ee3df09313f38e91393bfe2966e947c31e4002",
		"s": "0x9e105efcad78b8e836aa9c588e39f0d81b2d6552d04762d0e02652a9ea94b1d"
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
		"isCreate": false,
		"v": "0x19ece",
		"r": "0xa0ed5a985f5b74215ba05b0c3fc2a2af1c26c65d9426867eda637fa5d7d388eb",
		"s": "0x81054ba4a31ee6f0715f36d1005393623b97703c061afe5518a7e31ecbfda6f"		
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

type l1DataTestCase struct {
	TxDataSample      string
	EIP1559BaseFee    *big.Int
	L1basefee         *big.Int
	L1feeOverHead     *big.Int
	L1feeScalar       *big.Int
	EncodedExpected   int
	L1DataFeeExpected *big.Int
}

func testCalculateL1DataSize(t *testing.T, t_case *l1DataTestCase) {
	txdata := new(types.TransactionData)
	assert.NoError(t, json.Unmarshal([]byte(t_case.TxDataSample), txdata), "parse json fail")

	// we have decomposed CalcDataFeeForMessage here so
	// to catch more detail inside the process
	var msg types.Message

	if t_case.EIP1559BaseFee != nil {
		//TODO: EIP1559 test
		panic("no implement")
	} else {
		msg = reverseDataToMsg(txdata)
	}
	chainID := (*big.Int)(txdata.ChainId)
	signer := types.NewLondonSigner(chainID)
	unsigned := asUnsignedTx(msg, t_case.EIP1559BaseFee, chainID)

	// here we have to recover signature
	v := big.NewInt(0).Sub(txdata.V.ToInt(), big.NewInt(chainID.Int64()*2)).Uint64() - 35
	assert.True(t, v == 0 || v == 1, "v must be reasonably recovered")

	r := make([]byte, 32)
	s := make([]byte, 32)
	txdata.R.ToInt().FillBytes(r)
	txdata.S.ToInt().FillBytes(s)

	tx, err := unsigned.WithSignature(signer, bytes.Join([][]byte{r, s, {byte(v)}}, nil))
	assert.NoError(t, err, "build dummy tx fail")
	raw, err := rlpEncode(tx)
	assert.NoError(t, err, "rlp fail")

	if t_case.EncodedExpected != 0 {
		assert.Equal(t, t_case.EncodedExpected, len(raw))
	} else {
		t.Log("caluldated encoded rlp len:", len(raw))
	}

	l1DataFee := calculateEncodedL1DataFee(raw, t_case.L1feeOverHead, t_case.L1basefee, t_case.L1feeScalar)
	if t_case.L1DataFeeExpected != nil {
		assert.Equal(t, t_case.L1DataFeeExpected, l1DataFee)
	} else {
		t.Log("calculated l1data fee:", l1DataFee)
	}
}

func TestCalculateL1DataSize(t *testing.T) {

	for _, tcase := range []*l1DataTestCase{
		{
			TxDataSample:      example_tx1,
			L1basefee:         big.NewInt(0x64),
			L1feeOverHead:     big.NewInt(0x17d4),
			L1feeScalar:       big.NewInt(0x4a42fc80),
			EncodedExpected:   173,
			L1DataFeeExpected: big.NewInt(0xfffe8),
		},
		{
			TxDataSample:      example_tx2,
			L1basefee:         big.NewInt(0x64),
			L1feeOverHead:     big.NewInt(0x17d4),
			L1feeScalar:       big.NewInt(0x4a42fc80),
			EncodedExpected:   140,
			L1DataFeeExpected: big.NewInt(0xf3f2f),
		},
	} {
		testCalculateL1DataSize(t, tcase)
	}

}
