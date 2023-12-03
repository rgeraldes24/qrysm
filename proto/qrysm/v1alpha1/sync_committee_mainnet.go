//go:build !minimal

package zond

import (
	"github.com/prysmaticlabs/go-bitfield"
)

func NewSyncCommitteeParticipationBits() bitfield.Bitvector128 {
	return bitfield.NewBitvector128()
}

func ConvertToSyncContributionBitVector(b []byte) bitfield.Bitvector128 {
	return b
}
