//go:build circuit_capacity_checker

package circuitcapacitychecker

/*
#cgo LDFLAGS: -lm -ldl -lzkp -lzktrie
#include <stdlib.h>
#include "./libzkp/libzkp.h"
*/
import "C" //nolint:typecheck

import (
	"encoding/json"
	"sync"
	"unsafe"

	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/log"
)

func init() {
	C.init()
}

type CircuitCapacityChecker struct {
	sync.Mutex
	id uint64
}

func NewCircuitCapacityChecker() *CircuitCapacityChecker {
	id := C.new_circuit_capacity_checker()
	return &CircuitCapacityChecker{id: uint64(id)}
}

func (ccc *CircuitCapacityChecker) Reset() {
	ccc.Lock()
	defer ccc.Unlock()

	C.reset_circuit_capacity_checker(C.uint64_t(ccc.id))
}

func (ccc *CircuitCapacityChecker) ApplyTransaction(traces *types.BlockTrace) (uint64, error) {
	ccc.Lock()
	defer ccc.Unlock()

	tracesByt, err := json.Marshal(traces)
	if err != nil {
		return 0, ErrUnknown
	}

	tracesStr := C.CString(string(tracesByt))
	defer func() {
		C.free(unsafe.Pointer(tracesStr))
	}()

	log.Info("start to check circuit capacity for tx")
	result := C.apply_tx(C.uint64_t(ccc.id), tracesStr)
	log.Info("check circuit capacity for tx done")

	switch result {
	case 0:
		return 0, ErrUnknown
	case -1:
		return ErrBlockRowConsumptionOverflow
	case -2:
		return ErrTxRowConsumptionOverflow
	default:
		return result, nil
	}
}

func (ccc *CircuitCapacityChecker) ApplyBlock(traces *types.BlockTrace) (uint64, error) {
	txTraces := *traces
	for i, txTrace := range txTraces {
		txTrace.Transactions = traces.Transactions[i]
		txTrace.TxStorageTraces = traces.TxStorageTraces[i]
		txTrace.ExecutionResults = traces.ExecutionResults[i]
	}

	var result uint64
	var err error
	for i, txTrace := range txTraces {
		result, err = ccc.ApplyTransaction(&txTrace)
		if err == ErrTxRowConsumptionOverflow || err == ErrBlockRowConsumptionOverflow {
			return 0, ErrBlockRowConsumptionOverflow
		} else if err != nil {
			return 0, err
		}
	}
	return result, nil
}
