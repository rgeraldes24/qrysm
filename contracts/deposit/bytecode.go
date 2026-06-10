package deposit

import (
	_ "embed"
	"strings"
)

//go:embed bytecode.bin
var depositContractBin string

// DepositContractBin is the QRVM bytecode for the validator deposit contract.
var DepositContractBin = strings.TrimSpace(depositContractBin)
