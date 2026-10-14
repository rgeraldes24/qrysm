// Copyright 2019 Martin Holst Swende
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

package ops

import (
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/core/vm"
)

// TestSanity checks opcode names against go-qrl.
func TestSanity(t *testing.T) {

	for i := range 256 {
		// Lookup the name via opcode
		goqrlOp := vm.OpCode(byte(i))
		ourOp := OpCode(byte(i))
		{
			exp, got := goqrlOp.String(), ourOp.String()
			if exp != got {
				t.Errorf("op 0x%x, got %v expected %v", i, got, exp)
			}
		}
		// Lookup opcode via name
		if name := ourOp.String(); !strings.HasPrefix(name, "opcode") {
			our := byte(StringToOp(name))
			if byte(our) != byte(i) {
				t.Errorf("name %v, got 0x%x expected 0x%x", name, our, byte(i))
			}
			goqrl := byte(vm.StringToOp(name))
			if byte(goqrl) != byte(i) {
				t.Errorf("name %v, got 0x%x expected 0x%x", name, goqrl, byte(i))
			}
		}
	}
}

// This check can only be executed if the go-qrl codebase
// is refactored a bit, to make the following public.
//
//	func LookupInstructionSet(fork string) func() JumpTable
//	func (op *operation) Stack() (int, int)
//	func (op *operation) Valid() bool

func TestForkOpcodes(t *testing.T) {
	testForkOpcodes(t, "Zond")
}
func testForkOpcodes(t *testing.T, fork string) {
	var (
		f  *Fork
		jt vm.JumpTable
	)
	if f = LookupFork(fork); f == nil {
		t.Fatalf("fork missing %v", fork)
	}
	rules := LookupRules(fork)

	if _jt, err := vm.LookupInstructionSet(rules); err != nil {
		t.Fatalf("error lookup up jumptable in geth: %v", err)
	} else {
		jt = _jt
	}
	for op, gethOp := range jt {
		if !gethOp.HasCost() {
			continue
		}
		found := false
		for _, ourOp := range f.ValidOpcodes {
			if int(ourOp) == op {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing operation %#x %v in fork %v", op, vm.OpCode(op), fork)
		}
	}
	for _, ourOp := range f.ValidOpcodes {
		goqrlOp := vm.OpCode(ourOp)
		{
			exp, got := goqrlOp.String(), ourOp.String()
			if exp != got {
				t.Errorf("op got %v expected %v", got, exp)
			}
		}
		gotPops := len(ourOp.Pops())
		goqrlInstr := jt[goqrlOp]
		min, max := goqrlInstr.Stack()

		if gotPops != min {
			t.Errorf("op %v pops wrong, us: %d, go-qrl: %d", ourOp.String(), gotPops, min)
		}
		havePush := len(ourOp.Pushes())
		wantPush := 1024 - max + min
		if havePush != wantPush {
			t.Errorf("op %v push wrong, us: %d, go-qrl: %d", ourOp.String(), havePush, wantPush)
		}
	}
}
