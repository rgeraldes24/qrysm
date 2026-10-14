package execution

import (
	"github.com/pkg/errors"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/qrysm/config/params"
)

// DepositContractAddress returns the deposit contract address for the given chain.
func DepositContractAddress() (string, error) {
	address := params.BeaconConfig().DepositContractAddress
	if address == "" {
		return "", errors.New("valid deposit contract is required")
	}

	if !common.IsAddress(address) {
		return "", errors.New("invalid deposit contract address given: " + address)
	}
	return address, nil
}
