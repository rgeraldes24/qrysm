package state_native

import (
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/beacon-chain/state"
	testtmpl "github.com/theQRL/qrysm/beacon-chain/state/testing"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

func TestBeaconState_PreviousJustifiedCheckpointNil_Capella(t *testing.T) {
	testtmpl.VerifyBeaconStatePreviousJustifiedCheckpointNil(
		t,
		func() (state.BeaconState, error) {
			return InitializeFromProtoUnsafeCapella(&qrysmpb.BeaconStateCapella{})
		})
}

func TestBeaconState_PreviousJustifiedCheckpoint_Capella(t *testing.T) {
	testtmpl.VerifyBeaconStatePreviousJustifiedCheckpoint(
		t,
		func(cp *qrysmpb.Checkpoint) (state.BeaconState, error) {
			return InitializeFromProtoUnsafeCapella(&qrysmpb.BeaconStateCapella{PreviousJustifiedCheckpoint: cp})
		})
}

func TestBeaconState_CurrentJustifiedCheckpointNil_Capella(t *testing.T) {
	testtmpl.VerifyBeaconStateCurrentJustifiedCheckpointNil(
		t,
		func() (state.BeaconState, error) {
			return InitializeFromProtoUnsafeCapella(&qrysmpb.BeaconStateCapella{})
		})
}

func TestBeaconState_CurrentJustifiedCheckpoint_Capella(t *testing.T) {
	testtmpl.VerifyBeaconStateCurrentJustifiedCheckpoint(
		t,
		func(cp *qrysmpb.Checkpoint) (state.BeaconState, error) {
			return InitializeFromProtoUnsafeCapella(&qrysmpb.BeaconStateCapella{CurrentJustifiedCheckpoint: cp})
		})
}

func TestBeaconState_FinalizedCheckpointNil_Capella(t *testing.T) {
	testtmpl.VerifyBeaconStateFinalizedCheckpointNil(
		t,
		func() (state.BeaconState, error) {
			return InitializeFromProtoUnsafeCapella(&qrysmpb.BeaconStateCapella{})
		})
}

func TestBeaconState_FinalizedCheckpoint_Capella(t *testing.T) {
	testtmpl.VerifyBeaconStateFinalizedCheckpoint(
		t,
		func(cp *qrysmpb.Checkpoint) (state.BeaconState, error) {
			return InitializeFromProtoUnsafeCapella(&qrysmpb.BeaconStateCapella{FinalizedCheckpoint: cp})
		})
}

func TestBeaconState_JustificationBitsNil_Capella(t *testing.T) {
	testtmpl.VerifyBeaconStateJustificationBitsNil(
		t,
		func() (state.BeaconState, error) {
			return InitializeFromProtoUnsafeCapella(&qrysmpb.BeaconStateCapella{})
		})
}

func TestBeaconState_JustificationBits_Capella(t *testing.T) {
	testtmpl.VerifyBeaconStateJustificationBits(
		t,
		func(bits bitfield.Bitvector4) (state.BeaconState, error) {
			return InitializeFromProtoUnsafeCapella(&qrysmpb.BeaconStateCapella{JustificationBits: bits})
		})
}
