package blocks

import (
	"context"
	"testing"

	fuzz "github.com/google/gofuzz"
	v "github.com/theQRL/qrysm/beacon-chain/core/validators"
	state_native "github.com/theQRL/qrysm/beacon-chain/state/state-native"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/require"
)

func TestFuzzProcessBlockHeader_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	block := &qrysmpb.SignedBeaconBlockCapella{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(block)

		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		if block.Block == nil || block.Block.Body == nil || block.Block.Body.ExecutionData == nil {
			continue
		}
		wsb, err := blocks.NewSignedBeaconBlock(block)
		require.NoError(t, err)
		_, err = ProcessBlockHeader(context.Background(), s, wsb)
		_ = err
	}
}

func TestFuzzverifyDepositDataSigningRoot_10000(_ *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	var ba []byte
	var pubkey [field_params.DilithiumPubkeyLength]byte
	var sig [96]byte
	var domain [4]byte
	var p []byte
	var s []byte
	var d []byte
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(&ba)
		fuzzer.Fuzz(&pubkey)
		fuzzer.Fuzz(&sig)
		fuzzer.Fuzz(&domain)
		fuzzer.Fuzz(&p)
		fuzzer.Fuzz(&s)
		fuzzer.Fuzz(&d)
		err := verifySignature(ba, pubkey[:], sig[:], domain[:])
		_ = err
		err = verifySignature(ba, p, s, d)
		_ = err
	}
}

func TestFuzzProcessExecutionDataInBlock_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	e := &qrysmpb.ExecutionData{}
	state, err := state_native.InitializeFromProtoUnsafeCapella(&qrysmpb.BeaconStateCapella{})
	require.NoError(t, err)
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(e)
		s, err := ProcessExecutionDataInBlock(context.Background(), state, e)
		if err != nil && s != nil {
			t.Fatalf("state should be nil on err. found: %v on error: %v for state: %v and executiondata: %v", s, err, state, e)
		}
	}
}

func TestFuzzareExecutionDataEqual_10000(_ *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	executionData := &qrysmpb.ExecutionData{}
	executionData2 := &qrysmpb.ExecutionData{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(executionData)
		fuzzer.Fuzz(executionData2)
		AreExecutionDataEqual(executionData, executionData2)
		AreExecutionDataEqual(executionData, executionData)
	}
}

func TestFuzzExecutionDataHasEnoughSupport_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	executionData := &qrysmpb.ExecutionData{}
	var stateVotes []*qrysmpb.ExecutionData
	for i := 0; i < 100000; i++ {
		fuzzer.Fuzz(executionData)
		fuzzer.Fuzz(&stateVotes)
		s, err := state_native.InitializeFromProtoCapella(&qrysmpb.BeaconStateCapella{
			ExecutionDataVotes: stateVotes,
		})
		require.NoError(t, err)
		_, err = ExecutionDataHasEnoughSupport(s, executionData)
		_ = err
	}

}

func TestFuzzProcessBlockHeaderNoVerify_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	block := &qrysmpb.BeaconBlockCapella{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(block)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		_, err = ProcessBlockHeaderNoVerify(context.Background(), s, block.Slot, block.ProposerIndex, block.ParentRoot, []byte{})
		_ = err
	}
}

func TestFuzzProcessRandao_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	b := &qrysmpb.SignedBeaconBlockCapella{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(b)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		if b.Block == nil || b.Block.Body == nil {
			continue
		}
		wsb, err := blocks.NewSignedBeaconBlock(b)
		require.NoError(t, err)
		r, err := ProcessRandao(context.Background(), s, wsb)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, b)
		}
	}
}

func TestFuzzProcessRandaoNoVerify_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	blockBody := &qrysmpb.BeaconBlockBodyCapella{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(blockBody)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		r, err := ProcessRandaoNoVerify(s, blockBody.RandaoReveal)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, blockBody)
		}
	}
}

func TestFuzzProcessProposerSlashings_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	p := &qrysmpb.ProposerSlashing{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(p)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		r, err := ProcessProposerSlashings(ctx, s, []*qrysmpb.ProposerSlashing{p}, v.SlashValidator)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and slashing: %v", r, err, state, p)
		}
	}
}

func TestFuzzVerifyProposerSlashing_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	proposerSlashing := &qrysmpb.ProposerSlashing{}
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(proposerSlashing)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		err = VerifyProposerSlashing(s, proposerSlashing)
		_ = err
	}
}

func TestFuzzProcessAttesterSlashings_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	a := &qrysmpb.AttesterSlashing{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(a)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		r, err := ProcessAttesterSlashings(ctx, s, []*qrysmpb.AttesterSlashing{a}, v.SlashValidator)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and slashing: %v", r, err, state, a)
		}
	}
}

func TestFuzzVerifyAttesterSlashing_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	attesterSlashing := &qrysmpb.AttesterSlashing{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(attesterSlashing)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		err = VerifyAttesterSlashing(ctx, s, attesterSlashing)
		_ = err
	}
}

func TestFuzzIsSlashableAttestationData_10000(_ *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	attestationData := &qrysmpb.AttestationData{}
	attestationData2 := &qrysmpb.AttestationData{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(attestationData)
		fuzzer.Fuzz(attestationData2)
		IsSlashableAttestationData(attestationData, attestationData2)
	}
}

func TestFuzzslashableAttesterIndices_10000(_ *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	attesterSlashing := &qrysmpb.AttesterSlashing{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(attesterSlashing)
		SlashableAttesterIndices(attesterSlashing)
	}
}

func TestFuzzVerifyIndexedAttestationn_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	idxAttestation := &qrysmpb.IndexedAttestation{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(idxAttestation)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		err = VerifyIndexedAttestation(ctx, s, idxAttestation)
		_ = err
	}
}

func TestFuzzVerifyAttestation_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	attestation := &qrysmpb.Attestation{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(attestation)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		err = VerifyAttestationSignatures(ctx, s, attestation)
		_ = err
	}
}

func TestFuzzProcessDeposits_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	deposits := make([]*qrysmpb.Deposit, 100)
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		for i := range deposits {
			fuzzer.Fuzz(deposits[i])
		}
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		r, err := ProcessDeposits(ctx, s, deposits)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, deposits)
		}
	}
}

func TestFuzzProcessPreGenesisDeposit_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	deposit := &qrysmpb.Deposit{}
	ctx := context.Background()

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(deposit)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		r, err := ProcessPreGenesisDeposits(ctx, s, []*qrysmpb.Deposit{deposit})
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, deposit)
		}
	}
}

func TestFuzzProcessDeposit_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	deposit := &qrysmpb.Deposit{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(deposit)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		r, _, err := ProcessDeposit(s, deposit, true)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, deposit)
		}
	}
}

func TestFuzzverifyDeposit_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	deposit := &qrysmpb.Deposit{}
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(deposit)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		err = verifyDeposit(s, deposit)
		_ = err
	}
}

func TestFuzzProcessVoluntaryExits_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	e := &qrysmpb.SignedVoluntaryExit{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(e)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		r, err := ProcessVoluntaryExits(ctx, s, []*qrysmpb.SignedVoluntaryExit{e})
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and exit: %v", r, err, state, e)
		}
	}
}

func TestFuzzProcessVoluntaryExitsNoVerify_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &qrysmpb.BeaconStateCapella{}
	e := &qrysmpb.SignedVoluntaryExit{}
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(e)
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)
		r, err := ProcessVoluntaryExits(context.Background(), s, []*qrysmpb.SignedVoluntaryExit{e})
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, e)
		}
	}
}

func TestFuzzVerifyExit_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	ve := &qrysmpb.SignedVoluntaryExit{}
	rawVal := &qrysmpb.Validator{}
	fork := &qrysmpb.Fork{}
	var slot primitives.Slot

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(ve)
		fuzzer.Fuzz(rawVal)
		fuzzer.Fuzz(fork)
		fuzzer.Fuzz(&slot)

		state := &qrysmpb.BeaconStateCapella{
			Slot:                  slot,
			Fork:                  fork,
			GenesisValidatorsRoot: params.BeaconConfig().ZeroHash[:],
		}
		s, err := state_native.InitializeFromProtoUnsafeCapella(state)
		require.NoError(t, err)

		val, err := state_native.NewValidator(&qrysmpb.Validator{})
		_ = err
		err = VerifyExitAndSignature(val, s, ve)
		_ = err
	}
}
