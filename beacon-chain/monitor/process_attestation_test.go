package monitor

import (
	"bytes"
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func TestGetAttestingIndices(t *testing.T) {
	ctx := context.Background()
	beaconState, _ := util.DeterministicGenesisState(t, 256)
	att := &zondpb.Attestation{
		Data: &zondpb.AttestationData{
			Slot:           1,
			CommitteeIndex: 0,
		},
		ParticipationBits: bitfield.Bitlist{0b11, 0b1},
	}
	attestingIndices, err := attestingIndices(ctx, beaconState, att)
	require.NoError(t, err)
	require.DeepEqual(t, attestingIndices, []uint64{0x6e, 0xa5})
}

func TestProcessIncludedAttestationTwoTracked(t *testing.T) {
	hook := logTest.NewGlobal()
	s := setupService(t)
	state, _ := util.DeterministicGenesisState(t, 256)
	require.NoError(t, state.SetSlot(2))
	require.NoError(t, state.SetCurrentParticipationBits(bytes.Repeat([]byte{0xff}, 13)))

	att := &zondpb.Attestation{
		Data: &zondpb.AttestationData{
			Slot:            1,
			CommitteeIndex:  0,
			BeaconBlockRoot: bytesutil.PadTo([]byte("hello-world"), 32),
			Source: &zondpb.Checkpoint{
				Epoch: 0,
				Root:  bytesutil.PadTo([]byte("hello-world"), 32),
			},
			Target: &zondpb.Checkpoint{
				Epoch: 1,
				Root:  bytesutil.PadTo([]byte("hello-world"), 32),
			},
		},
		ParticipationBits: bitfield.Bitlist{0b11, 0b1},
	}
	s.processIncludedAttestation(context.Background(), state, att)
	wanted1 := "\"Attestation included\" BalanceChange=0 CorrectHead=false CorrectSource=false CorrectTarget=false Head=0x68656c6c6f2d InclusionSlot=2 NewBalance=32000000000 Slot=1 Source=0x68656c6c6f2d Target=0x68656c6c6f2d ValidatorIndex=110 prefix=monitor"
	wanted2 := "\"Attestation included\" BalanceChange=100000000 CorrectHead=false CorrectSource=false CorrectTarget=false Head=0x68656c6c6f2d InclusionSlot=2 NewBalance=32000000000 Slot=1 Source=0x68656c6c6f2d Target=0x68656c6c6f2d ValidatorIndex=165 prefix=monitor"
	require.LogsContain(t, hook, wanted1)
	require.LogsContain(t, hook, wanted2)
}

func TestProcessUnaggregatedAttestationStateNotCached(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	hook := logTest.NewGlobal()
	ctx := context.Background()

	s := setupService(t)
	state, _ := util.DeterministicGenesisState(t, 256)
	require.NoError(t, state.SetSlot(2))
	header := state.LatestBlockHeader()
	participation := []byte{0xff, 0xff, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	require.NoError(t, state.SetCurrentParticipationBits(participation))

	att := &zondpb.Attestation{
		Data: &zondpb.AttestationData{
			Slot:            1,
			CommitteeIndex:  0,
			BeaconBlockRoot: header.GetStateRoot(),
			Source: &zondpb.Checkpoint{
				Epoch: 0,
				Root:  bytesutil.PadTo([]byte("hello-world"), 32),
			},
			Target: &zondpb.Checkpoint{
				Epoch: 1,
				Root:  bytesutil.PadTo([]byte("hello-world"), 32),
			},
		},
		ParticipationBits: bitfield.Bitlist{0b11, 0b1},
	}
	s.processUnaggregatedAttestation(ctx, att)
	require.LogsContain(t, hook, "Skipping unaggregated attestation due to state not found in cache")
	logrus.SetLevel(logrus.InfoLevel)
}

func TestProcessUnaggregatedAttestationStateCached(t *testing.T) {
	ctx := context.Background()
	hook := logTest.NewGlobal()

	s := setupService(t)
	state, _ := util.DeterministicGenesisState(t, 256)
	participation := []byte{0xff, 0xff, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	require.NoError(t, state.SetCurrentParticipationBits(participation))

	var root [32]byte
	copy(root[:], "hello-world")

	att := &zondpb.Attestation{
		Data: &zondpb.AttestationData{
			Slot:            1,
			CommitteeIndex:  0,
			BeaconBlockRoot: root[:],
			Source: &zondpb.Checkpoint{
				Epoch: 0,
				Root:  root[:],
			},
			Target: &zondpb.Checkpoint{
				Epoch: 1,
				Root:  root[:],
			},
		},
		ParticipationBits: bitfield.Bitlist{0b11, 0b1},
	}
	require.NoError(t, s.config.StateGen.SaveState(ctx, root, state))
	s.processUnaggregatedAttestation(context.Background(), att)
	wanted1 := "\"Processed unaggregated attestation\" Head=0x68656c6c6f2d Slot=1 Source=0x68656c6c6f2d Target=0x68656c6c6f2d ValidatorIndex=110 prefix=monitor"
	wanted2 := "\"Processed unaggregated attestation\" Head=0x68656c6c6f2d Slot=1 Source=0x68656c6c6f2d Target=0x68656c6c6f2d ValidatorIndex=165 prefix=monitor"
	require.LogsContain(t, hook, wanted1)
	require.LogsContain(t, hook, wanted2)
}

func TestProcessAggregatedAttestationStateNotCached(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	hook := logTest.NewGlobal()
	ctx := context.Background()

	s := setupService(t)
	state, _ := util.DeterministicGenesisState(t, 256)
	require.NoError(t, state.SetSlot(2))
	header := state.LatestBlockHeader()
	participation := []byte{0xff, 0xff, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	require.NoError(t, state.SetCurrentParticipationBits(participation))

	att := &zondpb.AggregateAttestationAndProof{
		AggregatorIndex: 110,
		Aggregate: &zondpb.Attestation{
			Data: &zondpb.AttestationData{
				Slot:            1,
				CommitteeIndex:  0,
				BeaconBlockRoot: header.GetStateRoot(),
				Source: &zondpb.Checkpoint{
					Epoch: 0,
					Root:  bytesutil.PadTo([]byte("hello-world"), 32),
				},
				Target: &zondpb.Checkpoint{
					Epoch: 1,
					Root:  bytesutil.PadTo([]byte("hello-world"), 32),
				},
			},
			ParticipationBits: bitfield.Bitlist{0b11, 0b1},
		},
	}
	s.processAggregatedAttestation(ctx, att)
	require.LogsContain(t, hook, "\"Processed attestation aggregation\" AggregatorIndex=110 BeaconBlockRoot=0x000000000000 Slot=1 SourceRoot=0x68656c6c6f2d TargetRoot=0x68656c6c6f2d prefix=monitor")
	require.LogsContain(t, hook, "Skipping aggregated attestation due to state not found in cache")
	logrus.SetLevel(logrus.InfoLevel)
}

func TestProcessAggregatedAttestationStateCached(t *testing.T) {
	hook := logTest.NewGlobal()
	ctx := context.Background()
	s := setupService(t)
	state, _ := util.DeterministicGenesisState(t, 256)
	participation := []byte{0xff, 0xff, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	require.NoError(t, state.SetCurrentParticipationBits(participation))

	var root [32]byte
	copy(root[:], "hello-world")

	att := &zondpb.AggregateAttestationAndProof{
		AggregatorIndex: 110,
		Aggregate: &zondpb.Attestation{
			Data: &zondpb.AttestationData{
				Slot:            1,
				CommitteeIndex:  0,
				BeaconBlockRoot: root[:],
				Source: &zondpb.Checkpoint{
					Epoch: 0,
					Root:  root[:],
				},
				Target: &zondpb.Checkpoint{
					Epoch: 1,
					Root:  root[:],
				},
			},
			ParticipationBits: bitfield.Bitlist{0b10, 0b1},
		},
	}

	require.NoError(t, s.config.StateGen.SaveState(ctx, root, state))
	s.processAggregatedAttestation(ctx, att)
	require.LogsContain(t, hook, "\"Processed attestation aggregation\" AggregatorIndex=110 BeaconBlockRoot=0x68656c6c6f2d Slot=1 SourceRoot=0x68656c6c6f2d TargetRoot=0x68656c6c6f2d prefix=monitor")
	require.LogsContain(t, hook, "\"Processed aggregated attestation\" Head=0x68656c6c6f2d Slot=1 Source=0x68656c6c6f2d Target=0x68656c6c6f2d ValidatorIndex=165 prefix=monitor")
	require.LogsDoNotContain(t, hook, "\"Processed aggregated attestation\" Head=0x68656c6c6f2d Slot=1 Source=0x68656c6c6f2d Target=0x68656c6c6f2d ValidatorIndex=110 prefix=monitor")
}

func TestProcessAttestations(t *testing.T) {
	hook := logTest.NewGlobal()
	s := setupService(t)
	ctx := context.Background()
	state, _ := util.DeterministicGenesisState(t, 256)
	require.NoError(t, state.SetSlot(2))
	require.NoError(t, state.SetCurrentParticipationBits(bytes.Repeat([]byte{0xff}, 13)))

	att := &zondpb.Attestation{
		Data: &zondpb.AttestationData{
			Slot:            1,
			CommitteeIndex:  0,
			BeaconBlockRoot: bytesutil.PadTo([]byte("hello-world"), 32),
			Source: &zondpb.Checkpoint{
				Epoch: 0,
				Root:  bytesutil.PadTo([]byte("hello-world"), 32),
			},
			Target: &zondpb.Checkpoint{
				Epoch: 1,
				Root:  bytesutil.PadTo([]byte("hello-world"), 32),
			},
		},
		ParticipationBits: bitfield.Bitlist{0b11, 0b1},
	}

	block := &zondpb.BeaconBlock{
		Slot: 2,
		Body: &zondpb.BeaconBlockBody{
			Attestations: []*zondpb.Attestation{att},
		},
	}

	wrappedBlock, err := blocks.NewBeaconBlock(block)
	require.NoError(t, err)
	s.processAttestations(ctx, state, wrappedBlock)
	wanted1 := "\"Attestation included\" BalanceChange=0 CorrectHead=false CorrectSource=false CorrectTarget=false Head=0x68656c6c6f2d InclusionSlot=2 NewBalance=32000000000 Slot=1 Source=0x68656c6c6f2d Target=0x68656c6c6f2d ValidatorIndex=110 prefix=monitor"
	wanted2 := "\"Attestation included\" BalanceChange=100000000 CorrectHead=false CorrectSource=false CorrectTarget=false Head=0x68656c6c6f2d InclusionSlot=2 NewBalance=32000000000 Slot=1 Source=0x68656c6c6f2d Target=0x68656c6c6f2d ValidatorIndex=165 prefix=monitor"
	require.LogsContain(t, hook, wanted1)
	require.LogsContain(t, hook, wanted2)

}
