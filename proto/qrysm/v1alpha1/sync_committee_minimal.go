//go:build minimal

package zond

import (
	"github.com/theQRL/go-bitfield"
)

func NewSyncCommitteeParticipationBits() bitfield.Bitvector8 {
	return bitfield.NewBitvector8()
}

func ConvertToSyncContributionBitVector(b []byte) bitfield.Bitvector8 {
	return b
}
