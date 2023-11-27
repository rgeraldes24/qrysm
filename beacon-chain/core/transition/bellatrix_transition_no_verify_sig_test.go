package transition_test

import (
	"context"
	"math"
	"testing"

	"github.com/theQRL/qrysm/v4/beacon-chain/core/altair"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func TestProcessEpoch_BadBalanceBellatrix(t *testing.T) {
	s, _ := util.DeterministicGenesisStateBellatrix(t, 100)
	assert.NoError(t, s.SetSlot(63))
	assert.NoError(t, s.UpdateBalancesAtIndex(0, math.MaxUint64))
	participation := byte(0)
	participation, err := altair.AddValidatorFlag(participation, params.BeaconConfig().TimelyHeadFlagIndex)
	require.NoError(t, err)
	participation, err = altair.AddValidatorFlag(participation, params.BeaconConfig().TimelySourceFlagIndex)
	require.NoError(t, err)
	participation, err = altair.AddValidatorFlag(participation, params.BeaconConfig().TimelyTargetFlagIndex)
	require.NoError(t, err)

	epochParticipation, err := s.CurrentEpochParticipation()
	assert.NoError(t, err)
	epochParticipation[0] = participation
	assert.NoError(t, s.SetCurrentParticipationBits(epochParticipation))
	assert.NoError(t, s.SetPreviousParticipationBits(epochParticipation))
	_, err = altair.ProcessEpoch(context.Background(), s)
	assert.ErrorContains(t, "addition overflows", err)
}

func createFullBellatrixBlockWithOperations(t *testing.T) (state.BeaconState,
	*zondpb.SignedBeaconBlockBellatrix) {
	_, altairBlk := createFullAltairBlockWithOperations(t)
	blk := &zondpb.SignedBeaconBlockBellatrix{
		Block: &zondpb.BeaconBlockBellatrix{
			Slot:          altairBlk.Block.Slot,
			ProposerIndex: altairBlk.Block.ProposerIndex,
			ParentRoot:    altairBlk.Block.ParentRoot,
			StateRoot:     altairBlk.Block.StateRoot,
			Body: &zondpb.BeaconBlockBodyBellatrix{
				RandaoReveal:      altairBlk.Block.Body.RandaoReveal,
				Zond1Data:         altairBlk.Block.Body.Zond1Data,
				Graffiti:          altairBlk.Block.Body.Graffiti,
				ProposerSlashings: altairBlk.Block.Body.ProposerSlashings,
				AttesterSlashings: altairBlk.Block.Body.AttesterSlashings,
				Attestations:      altairBlk.Block.Body.Attestations,
				Deposits:          altairBlk.Block.Body.Deposits,
				VoluntaryExits:    altairBlk.Block.Body.VoluntaryExits,
				SyncAggregate:     altairBlk.Block.Body.SyncAggregate,
				ExecutionPayload: &enginev1.ExecutionPayload{
					ParentHash:    make([]byte, fieldparams.RootLength),
					FeeRecipient:  make([]byte, fieldparams.FeeRecipientLength),
					StateRoot:     make([]byte, fieldparams.RootLength),
					ReceiptsRoot:  make([]byte, fieldparams.RootLength),
					LogsBloom:     make([]byte, fieldparams.LogsBloomLength),
					PrevRandao:    make([]byte, fieldparams.RootLength),
					BaseFeePerGas: bytesutil.PadTo([]byte{1, 2, 3, 4}, fieldparams.RootLength),
					BlockHash:     make([]byte, fieldparams.RootLength),
					Transactions:  make([][]byte, 0),
					ExtraData:     make([]byte, 0),
				},
			},
		},
		Signature: nil,
	}
	beaconState, _ := util.DeterministicGenesisStateBellatrix(t, 32)
	return beaconState, blk
}

func createFullCapellaBlockWithOperations(t *testing.T) (state.BeaconState,
	*zondpb.SignedBeaconBlock) {
	_, bellatrixBlk := createFullBellatrixBlockWithOperations(t)
	blk := &zondpb.SignedBeaconBlockCapella{
		Block: &zondpb.BeaconBlockCapella{
			Slot:          bellatrixBlk.Block.Slot,
			ProposerIndex: bellatrixBlk.Block.ProposerIndex,
			ParentRoot:    bellatrixBlk.Block.ParentRoot,
			StateRoot:     bellatrixBlk.Block.StateRoot,
			Body: &zondpb.BeaconBlockBodyCapella{
				RandaoReveal:      bellatrixBlk.Block.Body.RandaoReveal,
				Zond1Data:         bellatrixBlk.Block.Body.Zond1Data,
				Graffiti:          bellatrixBlk.Block.Body.Graffiti,
				ProposerSlashings: bellatrixBlk.Block.Body.ProposerSlashings,
				AttesterSlashings: bellatrixBlk.Block.Body.AttesterSlashings,
				Attestations:      bellatrixBlk.Block.Body.Attestations,
				Deposits:          bellatrixBlk.Block.Body.Deposits,
				VoluntaryExits:    bellatrixBlk.Block.Body.VoluntaryExits,
				SyncAggregate:     bellatrixBlk.Block.Body.SyncAggregate,
				ExecutionPayload: &enginev1.ExecutionPayloadCapella{
					ParentHash:    make([]byte, fieldparams.RootLength),
					FeeRecipient:  make([]byte, fieldparams.FeeRecipientLength),
					StateRoot:     make([]byte, fieldparams.RootLength),
					ReceiptsRoot:  make([]byte, fieldparams.RootLength),
					LogsBloom:     make([]byte, fieldparams.LogsBloomLength),
					PrevRandao:    make([]byte, fieldparams.RootLength),
					BaseFeePerGas: bytesutil.PadTo([]byte{1, 2, 3, 4}, fieldparams.RootLength),
					BlockHash:     make([]byte, fieldparams.RootLength),
					Transactions:  make([][]byte, 0),
					Withdrawals:   make([]*enginev1.Withdrawal, 0),
					ExtraData:     make([]byte, 0),
				},
			},
		},
		Signature: nil,
	}
	beaconState, _ := util.DeterministicGenesisStateCapella(t, 32)
	return beaconState, blk
}
