//go:build circuit_capacity_checker

// // in repo_root_dir
// make libzkp
// sudo cp ./rollup/circuitcapacitychecker/libzkp/libzkp.so /usr/local/lib/
// sudo cp ./rollup/circuitcapacitychecker/libzkp/libzktrie.so /usr/local/lib/
// export LD_LIBRARY_PATH=/usr/local/lib/

// // in this dir
// go test -v -race -gcflags="-l" -ldflags="-s=false" -tags circuit_capacity_checker ./...
package circuitcapacitychecker

import (
    "encoding/json"
    "flag"
    "io"
    "os"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/scroll-tech/go-ethereum/core/types"
)

var (
    tracePath    = flag.String("trace", "test_trace.json", "trace")
)

func TestApplyTx(t *testing.T) {
    as := assert.New(t)
    ccc := NewCircuitCapacityChecker()

    trace := readBlockTrace(*tracePath, as)

    _, err := ccc.ApplyTransaction(trace)
    as.NoError(err)
}


func readBlockTrace(filePat string, as *assert.Assertions) *types.BlockTrace {
    f, err := os.Open(filePat)
    as.NoError(err)
    byt, err := io.ReadAll(f)
    as.NoError(err)

    trace := &types.BlockTrace{}
    as.NoError(json.Unmarshal(byt, trace))

    return trace
}