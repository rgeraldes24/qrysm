package interop

import (
	"context"
	"testing"

	state_native "github.com/theQRL/qrysm/beacon-chain/state/state-native"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/container/trie"
	enginev1 "github.com/theQRL/qrysm/proto/engine/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/require"
)

func TestGenerateGenesisStateZond(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	if fieldparams.Preset == "minimal" {
		params.OverrideBeaconConfig(params.MinimalSpecConfig().Copy())
	}

	ep := &enginev1.ExecutionPayloadZond{
		ParentHash:    make([]byte, 32),
		FeeRecipient:  make([]byte, fieldparams.FeeRecipientLength),
		StateRoot:     make([]byte, 32),
		ReceiptsRoot:  make([]byte, 32),
		LogsBloom:     make([]byte, 256),
		PrevRandao:    make([]byte, 32),
		BlockNumber:   0,
		GasLimit:      0,
		GasUsed:       0,
		Timestamp:     0,
		ExtraData:     make([]byte, 32),
		BaseFeePerGas: make([]byte, 32),
		BlockHash:     make([]byte, 32),
		Transactions:  make([][]byte, 0),
	}
	e1d := &qrysmpb.ExecutionData{
		DepositRoot:  make([]byte, 32),
		DepositCount: 0,
		BlockHash:    make([]byte, 32),
	}
	g, _, err := GenerateGenesisStateZond(context.Background(), 0, params.BeaconConfig().MinGenesisActiveValidatorCount, ep, e1d)
	require.NoError(t, err)
	// The interop genesis must carry participation arrays sized to the
	// validator registry, or the first attestation-bearing block fails to
	// process (index >= len(epochParticipation)).
	require.Equal(t, int(params.BeaconConfig().MinGenesisActiveValidatorCount), len(g.Validators))
	require.Equal(t, len(g.Validators), len(g.PreviousEpochParticipation), "PreviousEpochParticipation not sized to the validator registry")
	require.Equal(t, len(g.Validators), len(g.CurrentEpochParticipation), "CurrentEpochParticipation not sized to the validator registry")
	require.Equal(t, len(g.Validators), len(g.InactivityScores))

	tr, err := trie.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
	require.NoError(t, err)
	dr, err := tr.HashTreeRoot()
	require.NoError(t, err)
	g.ExecutionData.DepositRoot = dr[:]
	g.ExecutionData.BlockHash = make([]byte, 32)
	st, err := state_native.InitializeFromProtoUnsafeZond(g)
	require.NoError(t, err)
	_, err = st.MarshalSSZ()
	require.NoError(t, err)
}

// Nodes started with the same --interop-num-validators and genesis time must
// derive the same genesis. The deposit root is derived from the deposit data,
// signatures included, when the caller does not supply one.
func TestGenerateGenesisStateZond_Deterministic(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	if fieldparams.Preset == "minimal" {
		params.OverrideBeaconConfig(params.MinimalSpecConfig().Copy())
	}

	const numValidators = 4
	ep := &enginev1.ExecutionPayloadZond{
		ParentHash:    make([]byte, 32),
		FeeRecipient:  make([]byte, fieldparams.FeeRecipientLength),
		StateRoot:     make([]byte, 32),
		ReceiptsRoot:  make([]byte, 32),
		LogsBloom:     make([]byte, 256),
		PrevRandao:    make([]byte, 32),
		ExtraData:     make([]byte, 32),
		BaseFeePerGas: make([]byte, 32),
		BlockHash:     make([]byte, 32),
		Transactions:  make([][]byte, 0),
	}
	generate := func() (*qrysmpb.BeaconStateZond, []*qrysmpb.Deposit) {
		g, deposits, err := GenerateGenesisStateZond(context.Background(), 0, numValidators, ep, &qrysmpb.ExecutionData{BlockHash: make([]byte, 32)})
		require.NoError(t, err)
		require.Equal(t, numValidators, len(g.Validators))
		return g, deposits
	}
	g1, d1 := generate()
	g2, d2 := generate()

	require.Equal(t, len(d1), len(d2))
	for i := range d1 {
		require.DeepEqual(t, d1[i].Data.Signature, d2[i].Data.Signature, "deposit %d signature differs between runs", i)
	}
	require.DeepEqual(t, g1.ExecutionData.DepositRoot, g2.ExecutionData.DepositRoot)

	st1, err := state_native.InitializeFromProtoUnsafeZond(g1)
	require.NoError(t, err)
	st2, err := state_native.InitializeFromProtoUnsafeZond(g2)
	require.NoError(t, err)
	r1, err := st1.HashTreeRoot(context.Background())
	require.NoError(t, err)
	r2, err := st2.HashTreeRoot(context.Background())
	require.NoError(t, err)
	require.Equal(t, r1, r2)
}
