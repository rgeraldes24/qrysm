package validator

import (
	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/consensus-types/interfaces"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// Sets the ml-dsa-87 to exec data for a block.
func (vs *Server) setMLDSA87ToExecData(blk interfaces.SignedBeaconBlock, headState state.BeaconState) {
	if err := blk.SetMLDSA87ToExecutionChanges([]*qrysmpb.SignedMLDSA87ToExecutionChange{}); err != nil {
		log.WithError(err).Error("Could not set ml-dsa-87 to execution data in block")
		return
	}
	changes, err := vs.MLDSA87ChangesPool.MLDSA87ToExecChangesForInclusion(headState)
	if err != nil {
		log.WithError(err).Error("Could not get ml-dsa-87 to execution changes")
		return
	} else {
		if err := blk.SetMLDSA87ToExecutionChanges(changes); err != nil {
			log.WithError(err).Error("Could not set ml-dsa-87 to execution changes")
			return
		}
	}
}
