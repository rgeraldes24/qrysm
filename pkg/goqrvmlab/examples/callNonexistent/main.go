// Copyright 2019 Martin Holst Swende, Hubert Ritzdorf
// This file is part of the goevmlab library.
//
// The library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the goevmlab library. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/state"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/core/vm/runtime"
	"github.com/theQRL/go-qrl/params"
	common2 "github.com/theQRL/qrysm/pkg/goqrvmlab/common"
	"github.com/theQRL/qrysm/pkg/goqrvmlab/ops"
	"github.com/theQRL/qrysm/pkg/goqrvmlab/program"
)

func main() {

	if err := program.RunProgram(runit); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func staticCallAttack() []byte {
	// Causes 13928 staticcalls
	//30          39521949 ns/op // 39 ms
	//  9            112185165 ns/op // 112 ms
	a := program.NewProgram()
	dest := a.Jumpdest()
	reps := 800
	for i := 0; i < reps; i++ {
		a.Push(0)
		a.Op(ops.DUP1)
		a.Op(ops.DUP1)
		a.Op(ops.DUP1)
		a.Op(ops.GAS)
		a.Op(ops.GAS)
		a.Op(ops.STATICCALL)
		a.Op(ops.POP)
	}
	a.Jump(dest)
	return a.Bytecode()
}

func runit() error {
	code := staticCallAttack()

	aAddr := common.MustParseAddress("Q0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef01234567000000000000000000000000000000000000ff0a")
	alloc := make(core.GenesisAlloc)
	alloc[aAddr] = core.GenesisAccount{
		Nonce:   0,
		Code:    code,
		Balance: big.NewInt(0xffffffff),
	}
	var err error
	var (
		statedb, _ = state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
		sender     = common.MustParseAddress("Q0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef01234567000000000000000000000000000000000000feed")
	)
	for addr, acc := range alloc {
		statedb.CreateAccount(addr)
		statedb.SetCode(addr, acc.Code)
		statedb.SetNonce(addr, acc.Nonce)
		if acc.Balance != nil {
			statedb.SetBalance(addr, acc.Balance)
		}

	}
	statedb.CreateAccount(sender)

	runtimeConfig := runtime.Config{
		Origin:      sender,
		State:       statedb,
		GasLimit:    10000000,
		BlockNumber: new(big.Int).SetUint64(1),
		ChainConfig: &params.ChainConfig{
			ChainID: big.NewInt(1),
		},
		QRVMConfig: vm.Config{
			Tracer: &dumbTracer{},
		},
	}
	// Run with tracing
	_, _, err = runtime.Call(aAddr, nil, &runtimeConfig)
	// Diagnose it
	runtimeConfig.QRVMConfig = vm.Config{}
	res := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			_, _, err = runtime.Call(aAddr, nil, &runtimeConfig)

		}
	})
	fmt.Print(res.String())
	fmt.Println()
	return err
}

type dumbTracer struct {
	common2.BasicTracer
	counter uint64
}

func (d *dumbTracer) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
	if op == vm.STATICCALL {
		d.counter++
	}
	if op == vm.EXTCODESIZE {
		d.counter++
	}
}

func (d *dumbTracer) CaptureStart(env *vm.QRVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	fmt.Printf("captureStart\n")
	fmt.Printf("	from: %v\n", from.Hex())
	fmt.Printf("	to: %v\n", to.Hex())
}

func (d *dumbTracer) CaptureEnd(output []byte, gasUsed uint64, err error) {
	fmt.Printf("\nCaptureEnd\n")
	fmt.Printf("Counter: %d\n", d.counter)
}
