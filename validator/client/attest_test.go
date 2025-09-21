package client

import (
	"context"
	"encoding/hex"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/async/event"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/config/features"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
	qrysmTime "github.com/theQRL/qrysm/time"
	"gopkg.in/d4l3k/messagediff.v1"
)

func TestRequestAttestation_ValidatorDutiesRequestFailure(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, _, validatorKey, finish := setup(t)
	validator.duties = &qrysmpb.DutiesResponse{CurrentEpochDuties: []*qrysmpb.DutiesResponse_Duty{}}
	defer finish()

	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsContain(t, hook, "Could not fetch validator assignment")
}

func TestAttestToBlockHead_SubmitAttestation_EmptyCommittee(t *testing.T) {
	hook := logTest.NewGlobal()

	validator, _, validatorKey, finish := setup(t)
	defer finish()
	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &qrysmpb.DutiesResponse{CurrentEpochDuties: []*qrysmpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 0,
			Committee:      make([]primitives.ValidatorIndex, 0),
			ValidatorIndex: 0,
		}}}
	validator.SubmitAttestation(context.Background(), 0, pubKey)
	require.LogsContain(t, hook, "Empty committee")
}

func TestAttestToBlockHead_SubmitAttestation_RequestFailure(t *testing.T) {
	hook := logTest.NewGlobal()

	validator, m, validatorKey, finish := setup(t)
	defer finish()
	validator.duties = &qrysmpb.DutiesResponse{CurrentEpochDuties: []*qrysmpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      make([]primitives.ValidatorIndex, 111),
			ValidatorIndex: 0,
		}}}
	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.AttestationDataRequest{}),
	).Return(&qrysmpb.AttestationData{
		BeaconBlockRoot: make([]byte, fieldparams.RootLength),
		Target:          &qrysmpb.Checkpoint{Root: make([]byte, fieldparams.RootLength)},
		Source:          &qrysmpb.Checkpoint{Root: make([]byte, fieldparams.RootLength)},
	}, nil)
	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch2
	).Times(2).Return(&qrysmpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)
	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.Attestation{}),
	).Return(nil, errors.New("something went wrong"))

	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsContain(t, hook, "Could not submit attestation to beacon node")
}

func TestAttestToBlockHead_AttestsCorrectly(t *testing.T) {
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	hook := logTest.NewGlobal()
	validatorIndex := primitives.ValidatorIndex(7)
	committee := []primitives.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &qrysmpb.DutiesResponse{CurrentEpochDuties: []*qrysmpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}

	beaconBlockRoot := bytesutil.ToBytes32([]byte("A"))
	targetRoot := bytesutil.ToBytes32([]byte("B"))
	sourceRoot := bytesutil.ToBytes32([]byte("C"))
	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.AttestationDataRequest{}),
	).Return(&qrysmpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot[:],
		Target:          &qrysmpb.Checkpoint{Root: targetRoot[:]},
		Source:          &qrysmpb.Checkpoint{Root: sourceRoot[:], Epoch: 3},
	}, nil)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(2).Return(&qrysmpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil)

	var generatedAttestation *qrysmpb.Attestation
	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.Attestation{}),
	).Do(func(_ context.Context, att *qrysmpb.Attestation) {
		generatedAttestation = att
	}).Return(&qrysmpb.AttestResponse{}, nil)

	validator.SubmitAttestation(context.Background(), 30, pubKey)

	aggregationBitfield := bitfield.NewBitlist(uint64(len(committee)))
	aggregationBitfield.SetBitAt(4, true)
	expectedAttestation := &qrysmpb.Attestation{
		Data: &qrysmpb.AttestationData{
			BeaconBlockRoot: beaconBlockRoot[:],
			Target:          &qrysmpb.Checkpoint{Root: targetRoot[:]},
			Source:          &qrysmpb.Checkpoint{Root: sourceRoot[:], Epoch: 3},
		},
		AggregationBits: aggregationBitfield,
		Signatures:      [][]byte{make([]byte, 4627)},
	}

	root, err := signing.ComputeSigningRoot(expectedAttestation.Data, make([]byte, 32))
	require.NoError(t, err)

	sig, err := validator.keyManager.Sign(context.Background(), &validatorpb.SignRequest{
		PublicKey:   validatorKey.PublicKey().Marshal(),
		SigningRoot: root[:],
	})
	require.NoError(t, err)
	expectedAttestation.Signatures = [][]byte{sig.Marshal()}
	if !reflect.DeepEqual(generatedAttestation, expectedAttestation) {
		t.Errorf("Incorrectly attested head, wanted %v, received %v", expectedAttestation, generatedAttestation)
		diff, _ := messagediff.PrettyDiff(expectedAttestation, generatedAttestation)
		t.Log(diff)
	}
	require.LogsDoNotContain(t, hook, "Could not")
}

func TestAttestToBlockHead_BlocksDoubleAtt(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	validatorIndex := primitives.ValidatorIndex(7)
	committee := []primitives.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &qrysmpb.DutiesResponse{CurrentEpochDuties: []*qrysmpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	beaconBlockRoot := bytesutil.ToBytes32([]byte("A"))
	targetRoot := bytesutil.ToBytes32([]byte("B"))
	sourceRoot := bytesutil.ToBytes32([]byte("C"))
	beaconBlockRoot2 := bytesutil.ToBytes32([]byte("D"))

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.AttestationDataRequest{}),
	).Return(&qrysmpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot[:],
		Target:          &qrysmpb.Checkpoint{Root: targetRoot[:], Epoch: 4},
		Source:          &qrysmpb.Checkpoint{Root: sourceRoot[:], Epoch: 3},
	}, nil)
	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.AttestationDataRequest{}),
	).Return(&qrysmpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot2[:],
		Target:          &qrysmpb.Checkpoint{Root: targetRoot[:], Epoch: 4},
		Source:          &qrysmpb.Checkpoint{Root: sourceRoot[:], Epoch: 3},
	}, nil)
	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(4).Return(&qrysmpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.Attestation{}),
	).Return(&qrysmpb.AttestResponse{AttestationDataRoot: make([]byte, 32)}, nil /* error */)

	validator.SubmitAttestation(context.Background(), 30, pubKey)
	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsContain(t, hook, "Failed attestation slashing protection")
}

func TestAttestToBlockHead_BlocksSurroundAtt(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	validatorIndex := primitives.ValidatorIndex(7)
	committee := []primitives.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &qrysmpb.DutiesResponse{CurrentEpochDuties: []*qrysmpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	beaconBlockRoot := bytesutil.ToBytes32([]byte("A"))
	targetRoot := bytesutil.ToBytes32([]byte("B"))
	sourceRoot := bytesutil.ToBytes32([]byte("C"))

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.AttestationDataRequest{}),
	).Return(&qrysmpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot[:],
		Target:          &qrysmpb.Checkpoint{Root: targetRoot[:], Epoch: 2},
		Source:          &qrysmpb.Checkpoint{Root: sourceRoot[:], Epoch: 1},
	}, nil)
	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.AttestationDataRequest{}),
	).Return(&qrysmpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot[:],
		Target:          &qrysmpb.Checkpoint{Root: targetRoot[:], Epoch: 3},
		Source:          &qrysmpb.Checkpoint{Root: sourceRoot[:], Epoch: 0},
	}, nil)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(4).Return(&qrysmpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.Attestation{}),
	).Return(&qrysmpb.AttestResponse{}, nil /* error */)

	validator.SubmitAttestation(context.Background(), 30, pubKey)
	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsContain(t, hook, "Failed attestation slashing protection")
}

func TestAttestToBlockHead_BlocksSurroundedAtt(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	validatorIndex := primitives.ValidatorIndex(7)
	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	committee := []primitives.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	validator.duties = &qrysmpb.DutiesResponse{CurrentEpochDuties: []*qrysmpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	beaconBlockRoot := bytesutil.ToBytes32([]byte("A"))
	targetRoot := bytesutil.ToBytes32([]byte("B"))
	sourceRoot := bytesutil.ToBytes32([]byte("C"))

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.AttestationDataRequest{}),
	).Return(&qrysmpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot[:],
		Target:          &qrysmpb.Checkpoint{Root: targetRoot[:], Epoch: 3},
		Source:          &qrysmpb.Checkpoint{Root: sourceRoot[:], Epoch: 0},
	}, nil)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(4).Return(&qrysmpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.Attestation{}),
	).Return(&qrysmpb.AttestResponse{}, nil /* error */)

	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsDoNotContain(t, hook, failedAttLocalProtectionErr)

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.AttestationDataRequest{}),
	).Return(&qrysmpb.AttestationData{
		BeaconBlockRoot: bytesutil.PadTo([]byte("A"), 32),
		Target:          &qrysmpb.Checkpoint{Root: bytesutil.PadTo([]byte("B"), 32), Epoch: 2},
		Source:          &qrysmpb.Checkpoint{Root: bytesutil.PadTo([]byte("C"), 32), Epoch: 1},
	}, nil)

	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsContain(t, hook, "Failed attestation slashing protection")
}

func TestAttestToBlockHead_DoesNotAttestBeforeDelay(t *testing.T) {
	validator, m, validatorKey, finish := setup(t)
	defer finish()

	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.genesisTime = uint64(qrysmTime.Now().Unix())
	m.validatorClient.EXPECT().GetDuties(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.DutiesRequest{}),
	).Times(0)

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.AttestationDataRequest{}),
	).Times(0)

	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.Attestation{}),
	).Return(&qrysmpb.AttestResponse{}, nil /* error */).Times(0)

	timer := time.NewTimer(1 * time.Second)
	go validator.SubmitAttestation(context.Background(), 0, pubKey)
	<-timer.C
}

func TestAttestToBlockHead_DoesAttestAfterDelay(t *testing.T) {
	cfg := params.BeaconConfig().Copy()
	cfg.SecondsPerSlot = 10
	params.OverrideBeaconConfig(cfg)

	validator, m, validatorKey, finish := setup(t)
	defer finish()

	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()

	validator.genesisTime = uint64(qrysmTime.Now().Unix())
	validatorIndex := primitives.ValidatorIndex(5)
	committee := []primitives.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &qrysmpb.DutiesResponse{CurrentEpochDuties: []*qrysmpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		}}}

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.AttestationDataRequest{}),
	).Return(&qrysmpb.AttestationData{
		BeaconBlockRoot: bytesutil.PadTo([]byte("A"), 32),
		Target:          &qrysmpb.Checkpoint{Root: bytesutil.PadTo([]byte("B"), 32)},
		Source:          &qrysmpb.Checkpoint{Root: bytesutil.PadTo([]byte("C"), 32), Epoch: 3},
	}, nil).Do(func(arg0, arg1 interface{}) {
		wg.Done()
	})

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(2).Return(&qrysmpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil)

	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.Any(),
	).Return(&qrysmpb.AttestResponse{}, nil).Times(1)

	validator.SubmitAttestation(context.Background(), 0, pubKey)
}

func TestAttestToBlockHead_CorrectBitfieldLength(t *testing.T) {
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	validatorIndex := primitives.ValidatorIndex(2)
	committee := []primitives.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &qrysmpb.DutiesResponse{CurrentEpochDuties: []*qrysmpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		}}}
	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.AttestationDataRequest{}),
	).Return(&qrysmpb.AttestationData{
		Target:          &qrysmpb.Checkpoint{Root: bytesutil.PadTo([]byte("B"), 32)},
		Source:          &qrysmpb.Checkpoint{Root: bytesutil.PadTo([]byte("C"), 32), Epoch: 3},
		BeaconBlockRoot: make([]byte, fieldparams.RootLength),
	}, nil)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(2).Return(&qrysmpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	var generatedAttestation *qrysmpb.Attestation
	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&qrysmpb.Attestation{}),
	).Do(func(_ context.Context, att *qrysmpb.Attestation) {
		generatedAttestation = att
	}).Return(&qrysmpb.AttestResponse{}, nil /* error */)

	validator.SubmitAttestation(context.Background(), 30, pubKey)

	assert.Equal(t, 2, len(generatedAttestation.AggregationBits))
}

func TestSignAttestation(t *testing.T) {
	validator, m, _, finish := setup(t)
	defer finish()
	wantedFork := &qrysmpb.Fork{
		PreviousVersion: []byte{'a', 'b', 'c', 'd'},
		CurrentVersion:  []byte{'d', 'e', 'f', 'f'},
		Epoch:           0,
	}
	genesisValidatorsRoot := [32]byte{0x01, 0x02}
	attesterDomain, err := signing.Domain(wantedFork, 0, params.BeaconConfig().DomainBeaconAttester, genesisValidatorsRoot[:])
	require.NoError(t, err)
	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), gomock.Any()).
		Return(&qrysmpb.DomainResponse{SignatureDomain: attesterDomain}, nil)
	ctx := context.Background()
	att := util.NewAttestation()
	att.Data.Source.Epoch = 100
	att.Data.Target.Epoch = 200
	att.Data.Slot = 999
	att.Data.BeaconBlockRoot = bytesutil.PadTo([]byte("blockRoot"), 32)

	pk := testKeyFromBytes(t, []byte{1})
	validator.keyManager = newMockKeymanager(t, pk)
	sig, sr, err := validator.signAtt(ctx, pk.pub, att.Data, att.Data.Slot)
	require.NoError(t, err, "%x,%x,%v", sig, sr, err)
	require.Equal(t, "0c95bc1cd0aaca140728278fbf3b5ced5cfea88f464164dd146083b2d90dedddef2844116777bf87b56eee0990ee6e86f51be31007677e0bbab2d18f5060bdfcfedc90b930cc018966c410dc7cfc44bc23d18689f12a01280273218a471b3cd10bd3fb088a79a4f1f297b8213d93a019d56abf7e63c2a24f10db05a2bafde78e9cc47d0895e84d1e7009cf119fd1265379eca05d295f44e13eb5347f70b34393d854215d61ccda8980192ab09c1abe7bbff21865871a6f54a0222683e7db82239a709515c8e145e3147c5d806ac5e4d6a61e70703b6e3361b62adada8552192a719193916ea7a2aed8c717c842865edd45ebb6156c1b6e7616bcd27fab1dd35fe09a3e632e7542846df27d641de4ee1038cdb394310dbb8e6dba9f72e1fa181e1a82f325e5f99d61eb4159f4a0504483b6275356632e6c0bcaeef26c6635a1d01546787a2fefaf8ec3584bfd9a6eb0b04f6eaed414f6d6ba5f9a28f94e87316a04bad8e4c32bb63b63fb034605b2ff44ce5ddf5d17c6c764d653c20c0db8fe53811747765392242e954d2ca2852a67bd412556267844102da1b6e0dca3f9a8fbd82eca808d39e3cf47328377724491a821e50c2587219b4f9caa358406011e519c6acf17502494e35a411e839d2e89559217a628eb6d909880dc0e79b49703680b8a9ae359f73b6cea37fea7413d1be8a1b7b97b69d6ae9924547e4ef338f1e4951f40deeb255dc56ad94a9c8411764d3af3acba4244a8799f805a37542def41e1598aeaa75dd5cf8cc5485318795ce5a7fac05c03316d091e991d8663d9f8b365c0897642eaaeff4aa572176263b1eada7164f5a0b07a9718b0f7e6059ddc874e34d7632e1883fc2d19348d71243242062752873efb09fc6995cbd9f7af67ac63ea234d441f554df7a042feac4059ec611b975dd6d52b4b202df8248f591dd27eaac32a56390fe695d74944883866401511894366b5ebb799e43cefaa36041299404e2671f233e4317e78da380b6bee6919e19c3004e7ed37cc6162e707a14c1c9d3f80f7abd199f6035a92ddbc7c513ed51ce597c7e5cbd750da885df2b98e70627928fc61c1b4f30effd4a55a1db204f930180dd67c3b406d7824f18968957f44c972813c5f1474b75829e24030e2376768d3857b02c75af6611775e6530bcac7aba7abce2484c2dff4cbee50f24bd3bc3f2979ada9a979954d39c451e6c1b15b6140b8c8f8eab6c387e64e154c69f25aa807ecbb946a1c0d81c67995cc7ea1c7762f898cfbbd942bf909cae991d0c4726721facdc87157a2e30a81cc8004b967a7ad52e3d0a1937ed18642218de6d3f03b1cbab8676535a470b199bafea8bd15352fc5f988b2b8c675fc52a33fc1842ae25d71c5d4f0d7759aad9897e3b219cc261358eadabb36708e7d53599a5710f6548d018ccd5ecb56d476dd739b6acf391dbcdceebb87eedfaa82cef8178c5448fcca140dd7887e6fe4f46616d01456cf5f3b48593e8c0e040ae6f4dc6a772b78d0b721dc9424ad4166d730d303590b660e8409f877e497df59cd335a03baab6d8d49b9826501ef3dcf85e99c06136d242243cdd8b4d60cb90bfed3c535ebb9086509a9dd98df7ad386ec00454cc7ed2593f86cc8fd3b56d7fe3f6356cfa4587d1161e5f8db67ec0923614a7299ce33ee412779dde1843dcbebc868b20f0906cd64a553adeb716c5abf3108416a39dd1bd3aec8c75ff96295fef5d07ebbef6fd4703daa206b68a39bae3999d219083d38a6f3ae4d1fe366e2377f93c2c94e0f8c86f8027d22cd7d7c8d8658eaf1084ce63063451e222c263d030083c420120698eed13197f0b33a2e44d3a92028a0f03b38aa380b74b061baebdd1d98f641cc48221e8af5322ca8be2b1131b65f853c0f9869c8fe69b4d5bf0fb664fdba6d7552ff2b06297238065a5134dcc653a014afe7c6589613a025118d2343b4d776ca2c978f3ea19527a9cb0bee322cd899d6af28e3a433a6bcdc1a2de3b528b7424f1c9761cf99ac09b355818d822e9299bef519f98a3b7da720fe3e99acd488c7aa0dcb61c136abca0f121de239dcb34fb217ab58abdd8c4bc730409076faa7bab4d440801706fa45b6e86e4aa89f79fa927fc13a007e673c646cd7a12110ce69b61f7a7e0787575b23cd83b04c4dfb30ce94ce7d8d21a6dbf2958ff8723c0dc8a0017d580918af962c0496dd2f3446bb5fe4d5ada6067469b43fe78d2ec4cb247126f63f2691d42faf15716b9db72edcab2cdf89f7650f10f2d647325c732233050c8da392b8213acd9cab117f0592431e11da0e47de435b9063fb6a50c49a500ba5a01c4d3d06bfa995dbbfdc56b89ec70e03e8b4c64b3122f7b7723f664c4c4c20c93fc4463a0421d97fdb36433217e47839911440a51d7f4c88777dd92b49d5083b6fa68082aa5323b3460421535793451757c63d5a0394579617af696e601b60b10b1beee2c67e4015b363c6dc569acbab67d0bc7cb5944a533b0ccf286424c93c4790dda3858fed47b78e906b1b5ee2e6bb1f1d0e93f47b75b838b9ffbc4d82872a33df683cba94c3a133d6ffec75450d5add28490e489c538ac5d7cf54b2d4607c0669fed9b34dedc6da0c8f2f7405b985691b0b8346e0477abd36cd71c801d7a23295a9cb6d52e749af43a9ed34e8b1a020f394ba34bf10ef7b2e53c58830e22cd55ee7c5b8c0668e151aa43dd1fb32212c75822f09a32ce1066d42163122a611873fe12aa823dc9aae10a16468d20e5ad817f3cd68435e4d31c3c2c58e81b779fabe996e1f029f84fdfa48ab2380d19b35e5b97be36828592054ebba3765edb923c541098d7f735813c64e573b6415a0255a3ea6e8d8b918d01f5268e5b7b7fdd2d3469edad268b4337677c337c53bbb4ea4589f4305392acfe857d3b45158c25d82e436b49d7ddadbec85c942875667b6ea53e13b4b3b3fa03414b5eb46efefb52d0a3ebf57e700369d648acbb0802bbab70376b08ae990cbf614264ee8c5ad5d92c6b1b68dc6e9412d2ae22a48cc3ea4092f4d8f5f346ebb8903dcb6f389323547c0e6c6f98a89e30be8b96988839d85bc60afce0004068768a1fbb7bbd5dcdcf1bb28016128e18c70c24ec56358430163a24f8462aaa650c398a6869b78a078779fd618c0e83ca0d5ebb12901f773cc46f317a5f1f91133be0afca8d151532c41997d9828d58f46557c96e5da6477faa4453bdb2c76df4125bbbab8eca01e38645a8513f3f422b061f762f395d5505f8a95c3fff065a77ec172889b3aec3be8619486d56d36e48ca93eb64cef4d2c0c4cb1e04b7fbb04ab3321bd8b7c3babd25cf64f99936a2ee78304513e80b39970c6179e63617cd5faa6212481beec82d483f3b3a009010570d1aca8f5f7e64be165c6b943ce270bc2084add1c4956e0b850b015cb7c8628250bb4df51973539802c80e24691d386b4063dbedd64eca6afb5dc033b90b620db550023458f639c29c2c0f50cc815ea9b6dc00b987c6b9e639a2f7082f4546416f10186451cf5900fbaedd0250b5d02433c72668ad795c82037fe46fa8decc85052e976cd0e2b5b2ac3c4111e115302811d54e356a6f629b1128da0d74a536017311756172b2e30e0b80e616aef846ad9e21e26a9ebda13b45b6fe69a80c6a9e38495acadaa78e04eaa35b6988ff674337a863c291469cdfa9f92765b497a059eb5f81a80621549e670ccf719bbc9948460ff66b52411db9cfd0d1f75a6e1015e486c12de82d49026b02460caf0f1b8b813a5fb5b50d4b5ff324c6f80365d6be52302c9d7051f409e3a69e7596acde62288e15e1545c78c916160e264d475ee612c123fc961cd6f55d44fe87e7a6d1ccfe9a02b1cd26b8f34f75c3a8a8ba745e1d9c4af23a0edce04c00c8a510a9b04db17ce400d8ee748d18c38c57fe14efe2a127d917d9de28baa7b83a46eb9099ee0d614d18bb473c39d19ce00377aeba0b42f21913ab492255cf0d5c8cb469b3258b196d40e2480ace1aa1adaa8c92e34519e4074b4a295dfbee7a7ed7815ef897dda07987849f2c9f7d81ceef752729193c5a30b48c9b67a8b1f59ad52b46c4cc8a53f8b10609750ca15c051919c7b851d348054835d0d2872bd792c4216671c7e3a5a017fc367274c44c9560486d0946d89a4793273590e5e343628b7edecc36931bda0b2375026ccf6bf04221eb3725b8845a1c337689942dcc07daeef545bfb444b22397b01ae253e49b22cffa4517ae84042c46ff1223f74c1b6201c1858051e1ed379c01df7698db6cb143648d077435c0783a1f6594ab9b421ccbd68c4cffaa70ebddd6c35d251f1f3d20e7e25ae633340c2a8647625970b8cb0ca0044bf64519d9f86183034c5de00384ea9f866e0ec2261eaafe586f046609baf3ea0c0be21b9941a772bacc4ac3ef60249377c23e7a131b636d50c43b847b7fd0f7936ff88faa8d930948d09aa471acc999d265a4839b2be61bfb7c379bcf598801f72854707b8d58113c94e39afc7614f1a031f64a9887a97a701907d3ea527209178a7acd2c20e39f09eea75c6da64614e27422876c0c5477be986e8ad848f5313c7539c8ad9c28523b4328648e39bbfd9c763cb158b333fa01ff86a2f7c29428741db52999ca694652de3391999112e2170dd1d36c3d0918320c4cbdd183cf7a6e4d3ee0734a6ea04603a7a7159ca1ea5d3fb751f94e19b0bd5879f2328f542d83705bbc114b8e5649400d1b425dc3a2a1ced825381fc2b377cb2197168491b80b0d11cd2619ab81c4f2c2a420fa6ee35da7a1f43572e9583954b37408080eaff9c543bcc9a2b40a00dac58cb4ba423f38f25fc2d1ecca711948699d30f00d13445d5ca210bc917ac86a78e009603ca4fde061429b9b69b2bc759cbff3eb0c54a01a91261a574f796840d1121199529675aca9ba39fd98d35c948f7e22704451457f588a6ff79f1edebddb75b8db2200d35b6fd8ca01083669c291f29b674c38aae7ffc4a60189ed27f303e085e17ce90c349ad0a909d4a86cdadc2f38d44209ff27d9d19e45362150def8ec6ab53a5435d5bcefc3afb7327ba640db3176f434add328f789d37564c71120468c9ccf966d638e9f7b29f32e6c5bcb71896adda407bcad38159b187a0a34a360c26e04bc9fd5f179e7e1643b2540c73eb07980f68f49437544655c3a5da5c892446a39bf68a5440db1db436545c33754d07d975a470fbbb8f2fa04da4dc1e30463a660e42db01a2697b4c00d9b6c4349682ba8c1aa7acce73ce4ccf8237061792423c151863666704f42f1f24ef6d9db18cc4c84baf8dedb737ef65ee9419b92237ffc317ce54812117bd99628bf2f4533975a3e9d6b204bef3b697fdde2cf173bb9baeca8ab11c64eeb7f208e5a4cd8fdae349e986ea8e3c74b7ecdfdb0b88adeadb370dd36a37d4c67bc70ca4574127aa246a5c991b5520226cf31559466956c512c55abdc4373b2c050cecef4d179cf52a39d4435baa05ec6b343a78735592dda8a8fc910e2322943f6b6f1f3a3b0de25b7d7707a61d1ba1635c896e67db4f86cb7cbbfbd55a482e0b9753991d1155ba7624b80233490285a5443177d0b4c863afcca351514f62d2e043a89fdf1546769646f3564bc17fbf452d5194f73eb525f5754e3cfad0bc6097d315075d4d6f27a22df093ac6a472307f94e70f18953151d08c8840c447913df4c46b8c8d2af8a1efb5e8da378a3800ffd8f706603eab9ed6501ddf7f6526f72472361a2ffe2be8a3f8502b5a280b6030f7192e8c8336b59dd7c3fbb38f170a1309d7a384e782286a84e520414fde34d0dc06b757065aa4bbf66b1e3af3fdd0559fbfdf39eccc23356c435ab9e967174246b4d19319fc3fc2893b1fe5a1b7f773c6d3d2e22f6d50488b3d71164977eacfb0c1ec06f9ee4e2599b1c0df8684cef0d3748a16b806e41c754ed89e336226d5870aa17481c6a4ac99a41b65992a9a1b82144dea7a04b67cf0a6d730115b57dead2d5a5943f3145d77b863f1b89fa09007dafd0f7d7c4d636d1131085f8ec4e2685952332683e77eac37748b25792d50ad5249bf07bc97681fda2e26722972e5f80e7c69f6c3904ac1d21b0e8fe95f25acd6ab83de8181f26dc0a73b10e9d053a77fdfc437e32e52a19d9ee860a7350f5bd846ef9e0d4556befd28d2b5cda53b36ee3ea0d9378d57b4c7a34b67e63bb7e9a3872672cdaf0807eaeeec9ec10ccf8274f72683fd142817f115a012e433f73a5a4d528ce364fa9c5011854f9fb4bdf52ed2cf668225e19ae75801f8f0a81531df67f8c2329144acc5162413f2d4d4d8f582a6396258b58be526808a17262851aa1ec1d4f7e8f7ea743068d6796f8ea49d5772a8155d392541e6174b344802a4318a57ece1f626f6aa7bc5e2830fe0837e4d9034378a6b1ce7257a39c24363c6d868794b1c8f0082cb41a31acb9f82e323c40acade3f4336067969fa1a2a7c1df03232941598c9097a1a5c5182f399cbacfd40c10556bb4de0000000000000000000000000000000a0d121a242f363c", hex.EncodeToString(sig))
	// proposer domain
	require.DeepEqual(t, "02bbdb88056d6cbafd6e94575540"+
		"e74b8cf2c0f2c1b79b8e17e7b21ed1694305", hex.EncodeToString(sr[:]))
}

func TestServer_WaitToSlotOneThird_CanWait(t *testing.T) {
	cfg := params.BeaconConfig().Copy()
	cfg.SecondsPerSlot = 10
	params.OverrideBeaconConfig(cfg)

	currentTime := uint64(time.Now().Unix())
	currentSlot := primitives.Slot(4)
	genesisTime := currentTime - uint64(currentSlot.Mul(params.BeaconConfig().SecondsPerSlot))

	v := &validator{
		genesisTime: genesisTime,
		blockFeed:   new(event.Feed),
	}

	timeToSleep := params.BeaconConfig().SecondsPerSlot / 3
	oneThird := currentTime + timeToSleep
	v.waitOneThirdOrValidBlock(context.Background(), currentSlot)

	if oneThird != uint64(time.Now().Unix()) {
		t.Errorf("Wanted %d time for slot one third but got %d", oneThird, currentTime)
	}
}

func TestServer_WaitToSlotOneThird_SameReqSlot(t *testing.T) {
	currentTime := uint64(time.Now().Unix())
	currentSlot := primitives.Slot(4)
	genesisTime := currentTime - uint64(currentSlot.Mul(params.BeaconConfig().SecondsPerSlot))

	v := &validator{
		genesisTime:      genesisTime,
		blockFeed:        new(event.Feed),
		highestValidSlot: currentSlot,
	}

	v.waitOneThirdOrValidBlock(context.Background(), currentSlot)

	if currentTime != uint64(time.Now().Unix()) {
		t.Errorf("Wanted %d time for slot one third but got %d", uint64(time.Now().Unix()), currentTime)
	}
}

func TestServer_WaitToSlotOneThird_ReceiveBlockSlot(t *testing.T) {
	cfg := params.BeaconConfig().Copy()
	cfg.SecondsPerSlot = 10
	params.OverrideBeaconConfig(cfg)

	resetCfg := features.InitWithReset(&features.Flags{AttestTimely: true})
	defer resetCfg()

	currentTime := uint64(time.Now().Unix())
	currentSlot := primitives.Slot(4)
	genesisTime := currentTime - uint64(currentSlot.Mul(params.BeaconConfig().SecondsPerSlot))

	v := &validator{
		genesisTime: genesisTime,
		blockFeed:   new(event.Feed),
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		wsb, err := blocks.NewSignedBeaconBlock(
			&qrysmpb.SignedBeaconBlockCapella{
				Block: &qrysmpb.BeaconBlockCapella{Slot: currentSlot, Body: &qrysmpb.BeaconBlockBodyCapella{}},
			})
		require.NoError(t, err)
		v.blockFeed.Send(wsb)
		wg.Done()
	}()

	v.waitOneThirdOrValidBlock(context.Background(), currentSlot)

	if currentTime != uint64(time.Now().Unix()) {
		t.Errorf("Wanted %d time for slot one third but got %d", uint64(time.Now().Unix()), currentTime)
	}
}
