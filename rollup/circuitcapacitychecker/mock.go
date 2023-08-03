//go:build !circuit_capacity_checker

package circuitcapacitychecker

import (
	"math/rand"

	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/log"
)

type CircuitCapacityChecker struct {
	ID uint64
}

func NewCircuitCapacityChecker() *CircuitCapacityChecker {
	log.Info("using MOCK NewCircuitCapacityChecker")
	return &CircuitCapacityChecker{ID: rand.Uint64()}
}

func (ccc *CircuitCapacityChecker) Reset() {
}

func (ccc *CircuitCapacityChecker) ApplyTransaction(traces *types.BlockTrace) (*types.RowConsumption, error) {
	return nil, nil
}

func (ccc *CircuitCapacityChecker) ApplyBlock(traces *types.BlockTrace) (*types.RowConsumption, error) {
	return nil, nil
}
