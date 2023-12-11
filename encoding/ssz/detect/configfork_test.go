package detect

import (
	"context"
	"fmt"
	"testing"

	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/runtime/version"
	"github.com/theQRL/qrysm/v4/testing/util"

	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/require"
)

func TestSlotFromBlock(t *testing.T) {
	b := util.NewBeaconBlock()
	var slot primitives.Slot = 3
	b.Block.Slot = slot
	bb, err := b.MarshalSSZ()
	require.NoError(t, err)
	sfb, err := slotFromBlock(bb)
	require.NoError(t, err)
	require.Equal(t, slot, sfb)
}

func TestByState(t *testing.T) {
	// undo, err := hackCapellaMaxuint()
	// require.NoError(t, err)
	// defer func() {
	// 	require.NoError(t, undo())
	// }()
	bc := params.BeaconConfig()
	cases := []struct {
		name        string
		version     int
		slot        primitives.Slot
		forkversion [4]byte
	}{
		{
			name:        "genesis",
			version:     version.Capella,
			slot:        0,
			forkversion: bytesutil.ToBytes4(bc.GenesisForkVersion),
		},
	}
	for _, c := range cases {
		st, err := stateForVersion(c.version)
		require.NoError(t, err)
		require.NoError(t, st.SetFork(&zondpb.Fork{
			PreviousVersion: make([]byte, 4),
			CurrentVersion:  c.forkversion[:],
			Epoch:           0,
		}))
		require.NoError(t, st.SetSlot(c.slot))
		m, err := st.MarshalSSZ()
		require.NoError(t, err)
		cf, err := FromState(m)
		require.NoError(t, err)
		require.Equal(t, c.version, cf.Fork)
		require.Equal(t, c.forkversion, cf.Version)
		require.Equal(t, bc.ConfigName, cf.Config.ConfigName)
	}
}

func stateForVersion(v int) (state.BeaconState, error) {
	switch v {
	case version.Capella:
		return util.NewBeaconState()
	default:
		return nil, fmt.Errorf("unrecognized version %d", v)
	}
}

func TestUnmarshalState(t *testing.T) {
	ctx := context.Background()
	// undo, err := hackCapellaMaxuint()
	// require.NoError(t, err)
	// defer func() {
	// 	require.NoError(t, undo())
	// }()
	bc := params.BeaconConfig()
	// require.NoError(t, err)
	cases := []struct {
		name        string
		version     int
		slot        primitives.Slot
		forkversion [4]byte
	}{
		{
			name:        "genesis",
			version:     version.Capella,
			slot:        0,
			forkversion: bytesutil.ToBytes4(bc.GenesisForkVersion),
		},
	}
	for _, c := range cases {
		st, err := stateForVersion(c.version)
		require.NoError(t, err)
		require.NoError(t, st.SetFork(&zondpb.Fork{
			PreviousVersion: make([]byte, 4),
			CurrentVersion:  c.forkversion[:],
			Epoch:           0,
		}))
		require.NoError(t, st.SetSlot(c.slot))
		m, err := st.MarshalSSZ()
		require.NoError(t, err)
		cf, err := FromState(m)
		require.NoError(t, err)
		s, err := cf.UnmarshalBeaconState(m)
		require.NoError(t, err)
		expected, err := st.HashTreeRoot(ctx)
		require.NoError(t, err)
		actual, err := s.HashTreeRoot(ctx)
		require.NoError(t, err)
		require.DeepEqual(t, expected, actual)
	}
}

// func hackCapellaMaxuint() (func() error, error) {
// 	// We monkey patch the config to use a smaller value for the bellatrix fork epoch.
// 	// Upstream configs use MaxUint64, which leads to a multiplication overflow when converting epoch->slot.
// 	// Unfortunately we have unit tests that assert our config matches the upstream config, so we have to choose between
// 	// breaking conformance, adding a special case to the conformance unit test, or patch it here.
// 	bc := params.MainnetConfig().Copy()
// 	bc.CapellaForkEpoch = math.MaxUint32
// 	undo, err := params.SetActiveWithUndo(bc)
// 	return undo, err
// }

func TestUnmarshalBlock(t *testing.T) {
	// undo, err := hackCapellaMaxuint()
	// require.NoError(t, err)
	// defer func() {
	// 	require.NoError(t, undo())
	// }()
	genv := bytesutil.ToBytes4(params.BeaconConfig().GenesisForkVersion)
	cases := []struct {
		b       func(*testing.T, primitives.Slot) interfaces.ReadOnlySignedBeaconBlock
		name    string
		version [4]byte
		slot    primitives.Slot
		err     error
	}{
		{
			name:    "genesis - slot 0",
			b:       signedTestBlockCapella,
			version: genv,
		},
		/*
			{
				name:    "last slot of phase 0",
				b:       signedTestBlockGenesis,
				version: genv,
				slot:    altairS - 1,
			},
			{
				name:    "first slot of altair",
				b:       signedTestBlockAltair,
				version: altairv,
				slot:    altairS,
			},
			{
				name:    "last slot of altair",
				b:       signedTestBlockAltair,
				version: altairv,
				slot:    bellaS - 1,
			},
			{
				name:    "first slot of bellatrix",
				b:       signedTestBlockBellatrix,
				version: bellav,
				slot:    bellaS,
			},
			{
				name:    "bellatrix block in altair slot",
				b:       signedTestBlockBellatrix,
				version: bellav,
				slot:    bellaS - 1,
				err:     errBlockForkMismatch,
			},
			{
				name:    "genesis block in altair slot",
				b:       signedTestBlockGenesis,
				version: genv,
				slot:    bellaS - 1,
				err:     errBlockForkMismatch,
			},
			{
				name:    "altair block in genesis slot",
				b:       signedTestBlockAltair,
				version: altairv,
				err:     errBlockForkMismatch,
			},
		*/
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := c.b(t, c.slot)
			marshaled, err := b.MarshalSSZ()
			require.NoError(t, err)
			cf, err := FromForkVersion(c.version)
			require.NoError(t, err)
			bcf, err := cf.UnmarshalBeaconBlock(marshaled)
			if c.err != nil {
				require.ErrorIs(t, err, c.err)
				return
			}
			require.NoError(t, err)
			expected, err := b.Block().HashTreeRoot()
			require.NoError(t, err)
			actual, err := bcf.Block().HashTreeRoot()
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
	}
}

func TestUnmarshalBlindedBlock(t *testing.T) {
	// undo, err := hackCapellaMaxuint()
	// require.NoError(t, err)
	// defer func() {
	// 	require.NoError(t, undo())
	// }()
	genv := bytesutil.ToBytes4(params.BeaconConfig().GenesisForkVersion)
	// TODO (remove)
	// altairv := bytesutil.ToBytes4(params.BeaconConfig().AltairForkVersion)
	// bellav := bytesutil.ToBytes4(params.BeaconConfig().BellatrixForkVersion)
	// altairS, err := slots.EpochStart(params.BeaconConfig().AltairForkEpoch)
	// require.NoError(t, err)
	// bellaS, err := slots.EpochStart(params.BeaconConfig().BellatrixForkEpoch)
	// require.NoError(t, err)
	cases := []struct {
		b       func(*testing.T, primitives.Slot) interfaces.ReadOnlySignedBeaconBlock
		name    string
		version [4]byte
		slot    primitives.Slot
		err     error
	}{
		// {
		// 	name:    "genesis - slot 0",
		// 	b:       signedTestBlockCapella,
		// 	version: genv,
		// },
		{
			name:    "genesis - slot 0 blinded",
			b:       signedTestBlindedBlockCapella,
			version: genv,
		},
		// {
		// 	name:    "last slot of phase 0",
		// 	b:       signedTestBlockGenesis,
		// 	version: genv,
		// 	slot:    altairS - 1,
		// },
		// {
		// 	name:    "first slot of altair",
		// 	b:       signedTestBlockAltair,
		// 	version: altairv,
		// 	slot:    altairS,
		// },
		// {
		// 	name:    "last slot of altair",
		// 	b:       signedTestBlockAltair,
		// 	version: altairv,
		// 	slot:    bellaS - 1,
		// },
		// {
		// 	name:    "first slot of bellatrix",
		// 	b:       signedTestBlindedBlockBellatrix,
		// 	version: bellav,
		// 	slot:    bellaS,
		// },
		// {
		// 	name:    "bellatrix block in altair slot",
		// 	b:       signedTestBlindedBlockBellatrix,
		// 	version: bellav,
		// 	slot:    bellaS - 1,
		// 	err:     errBlockForkMismatch,
		// },
		// {
		// 	name:    "genesis block in altair slot",
		// 	b:       signedTestBlockGenesis,
		// 	version: genv,
		// 	slot:    bellaS - 1,
		// 	err:     errBlockForkMismatch,
		// },
		// {
		// 	name:    "altair block in genesis slot",
		// 	b:       signedTestBlockAltair,
		// 	version: altairv,
		// 	err:     errBlockForkMismatch,
		// },
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := c.b(t, c.slot)
			marshaled, err := b.MarshalSSZ()
			require.NoError(t, err)
			cf, err := FromForkVersion(c.version)
			require.NoError(t, err)
			bcf, err := cf.UnmarshalBlindedBeaconBlock(marshaled)
			if c.err != nil {
				require.ErrorIs(t, err, c.err)
				return
			}
			require.NoError(t, err)
			expected, err := b.Block().HashTreeRoot()
			require.NoError(t, err)
			actual, err := bcf.Block().HashTreeRoot()
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
	}
}

func signedTestBlockCapella(t *testing.T, slot primitives.Slot) interfaces.ReadOnlySignedBeaconBlock {
	b := util.NewBeaconBlock()
	b.Block.Slot = slot
	s, err := blocks.NewSignedBeaconBlock(b)
	require.NoError(t, err)
	return s
}

func signedTestBlindedBlockCapella(t *testing.T, slot primitives.Slot) interfaces.ReadOnlySignedBeaconBlock {
	b := util.NewBlindedBeaconBlock()
	b.Block.Slot = slot
	s, err := blocks.NewSignedBeaconBlock(b)
	require.NoError(t, err)
	return s
}
