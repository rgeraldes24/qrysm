package traces

import (
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/uint512"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/qrl/tracers/logger"
)

func TestDetermineDestinationUsesFullAddressWord(t *testing.T) {
	var dest common.Address
	dest[0] = 0x11
	dest[15] = 0x22
	dest[16] = 0x33
	dest[common.AddressLength-1] = 0x44

	var destWord uint512.Int
	destWord.SetBytes(dest[:])

	var current common.Address
	current[0] = 0xaa
	current[common.AddressLength-1] = 0xbb

	tests := []struct {
		name        string
		op          vm.OpCode
		wantContext common.Address
	}{
		{
			name:        "call",
			op:          vm.CALL,
			wantContext: dest,
		},
		{
			name:        "staticcall",
			op:          vm.STATICCALL,
			wantContext: dest,
		},
		{
			name:        "delegatecall",
			op:          vm.DELEGATECALL,
			wantContext: current,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &logger.StructLog{
				Op:    tt.op,
				Stack: []uint512.Int{*uint512.NewInt(0), destWord},
			}

			contextAddr, callDest, _ := determineDestination(log, &current)
			if callDest == nil {
				t.Fatal("missing call destination")
			}
			if *callDest != dest {
				t.Fatalf("call destination mismatch: got %s want %s", callDest.Hex(), dest.Hex())
			}
			if contextAddr == nil {
				t.Fatal("missing context address")
			}
			if *contextAddr != tt.wantContext {
				t.Fatalf("context address mismatch: got %s want %s", contextAddr.Hex(), tt.wantContext.Hex())
			}
		})
	}
}
