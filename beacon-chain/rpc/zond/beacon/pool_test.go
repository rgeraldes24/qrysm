package beacon

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/theQRL/go-bitfield"
	grpcutil "github.com/theQRL/qrysm/v4/api/grpc"
	blockchainmock "github.com/theQRL/qrysm/v4/beacon-chain/blockchain/testing"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/signing"
	qrysmtime "github.com/theQRL/qrysm/v4/beacon-chain/core/time"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/transition"
	"github.com/theQRL/qrysm/v4/beacon-chain/operations/attestations"
	"github.com/theQRL/qrysm/v4/beacon-chain/operations/dilithiumtoexec"
	dilithiumtoexecmock "github.com/theQRL/qrysm/v4/beacon-chain/operations/dilithiumtoexec/mock"
	slashingsmock "github.com/theQRL/qrysm/v4/beacon-chain/operations/slashings/mock"
	"github.com/theQRL/qrysm/v4/beacon-chain/operations/voluntaryexits/mock"
	p2pMock "github.com/theQRL/qrysm/v4/beacon-chain/p2p/testing"
	state_native "github.com/theQRL/qrysm/v4/beacon-chain/state/state-native"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	"github.com/theQRL/qrysm/v4/crypto/dilithium/common"
	"github.com/theQRL/qrysm/v4/crypto/hash"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/encoding/ssz"
	"github.com/theQRL/qrysm/v4/proto/migration"
	zondpbv1alpha1 "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	zondpbv1 "github.com/theQRL/qrysm/v4/proto/zond/v1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
	"github.com/theQRL/qrysm/v4/time/slots"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestListPoolAttestations(t *testing.T) {
	bs, err := util.NewBeaconState()
	require.NoError(t, err)
	att1 := &zondpbv1alpha1.Attestation{
		ParticipationBits: []byte{1, 10},
		Data: &zondpbv1alpha1.AttestationData{
			Slot:            1,
			CommitteeIndex:  1,
			BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot1"), 32),
			Source: &zondpbv1alpha1.Checkpoint{
				Epoch: 1,
				Root:  bytesutil.PadTo([]byte("sourceroot1"), 32),
			},
			Target: &zondpbv1alpha1.Checkpoint{
				Epoch: 10,
				Root:  bytesutil.PadTo([]byte("targetroot1"), 32),
			},
		},
		Signatures: [][]byte{bytesutil.PadTo([]byte("signature1"), 4595)},
	}
	att2 := &zondpbv1alpha1.Attestation{
		ParticipationBits: []byte{4, 40},
		Data: &zondpbv1alpha1.AttestationData{
			Slot:            4,
			CommitteeIndex:  4,
			BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot4"), 32),
			Source: &zondpbv1alpha1.Checkpoint{
				Epoch: 4,
				Root:  bytesutil.PadTo([]byte("sourceroot4"), 32),
			},
			Target: &zondpbv1alpha1.Checkpoint{
				Epoch: 40,
				Root:  bytesutil.PadTo([]byte("targetroot4"), 32),
			},
		},
		Signatures: [][]byte{bytesutil.PadTo([]byte("signature4"), 4595)},
	}
	att3 := &zondpbv1alpha1.Attestation{
		ParticipationBits: []byte{2, 20},
		Data: &zondpbv1alpha1.AttestationData{
			Slot:            2,
			CommitteeIndex:  2,
			BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot2"), 32),
			Source: &zondpbv1alpha1.Checkpoint{
				Epoch: 2,
				Root:  bytesutil.PadTo([]byte("sourceroot2"), 32),
			},
			Target: &zondpbv1alpha1.Checkpoint{
				Epoch: 20,
				Root:  bytesutil.PadTo([]byte("targetroot2"), 32),
			},
		},
		Signatures: [][]byte{bytesutil.PadTo([]byte("signature2"), 4595)},
	}
	att4 := &zondpbv1alpha1.Attestation{
		ParticipationBits: bitfield.NewBitlist(8),
		Data: &zondpbv1alpha1.AttestationData{
			Slot:            4,
			CommitteeIndex:  4,
			BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot2"), 32),
			Source: &zondpbv1alpha1.Checkpoint{
				Epoch: 2,
				Root:  bytesutil.PadTo([]byte("sourceroot2"), 32),
			},
			Target: &zondpbv1alpha1.Checkpoint{
				Epoch: 20,
				Root:  bytesutil.PadTo([]byte("targetroot2"), 32),
			},
		},
		Signatures: [][]byte{bytesutil.PadTo([]byte("signature2"), 4595)},
	}
	att5 := &zondpbv1alpha1.Attestation{
		ParticipationBits: bitfield.NewBitlist(8),
		Data: &zondpbv1alpha1.AttestationData{
			Slot:            2,
			CommitteeIndex:  4,
			BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot1"), 32),
			Source: &zondpbv1alpha1.Checkpoint{
				Epoch: 2,
				Root:  bytesutil.PadTo([]byte("sourceroot2"), 32),
			},
			Target: &zondpbv1alpha1.Checkpoint{
				Epoch: 20,
				Root:  bytesutil.PadTo([]byte("targetroot2"), 32),
			},
		},
		Signatures: [][]byte{bytesutil.PadTo([]byte("signature1"), 4595)},
	}
	att6 := &zondpbv1alpha1.Attestation{
		ParticipationBits: bitfield.NewBitlist(8),
		Data: &zondpbv1alpha1.AttestationData{
			Slot:            2,
			CommitteeIndex:  4,
			BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot2"), 32),
			Source: &zondpbv1alpha1.Checkpoint{
				Epoch: 2,
				Root:  bytesutil.PadTo([]byte("sourceroot2"), 32),
			},
			Target: &zondpbv1alpha1.Checkpoint{
				Epoch: 20,
				Root:  bytesutil.PadTo([]byte("targetroot2"), 32),
			},
		},
		Signatures: [][]byte{bytesutil.PadTo([]byte("signature2"), 4595)},
	}
	s := &Server{
		ChainInfoFetcher: &blockchainmock.ChainService{State: bs},
		AttestationsPool: attestations.NewPool(),
	}
	require.NoError(t, s.AttestationsPool.SaveAggregatedAttestations([]*zondpbv1alpha1.Attestation{att1, att2, att3}))
	require.NoError(t, s.AttestationsPool.SaveUnaggregatedAttestations([]*zondpbv1alpha1.Attestation{att4, att5, att6}))

	t.Run("empty request", func(t *testing.T) {
		req := &zondpbv1.AttestationsPoolRequest{}
		resp, err := s.ListPoolAttestations(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, 6, len(resp.Data))
	})

	t.Run("slot request", func(t *testing.T) {
		slot := primitives.Slot(2)
		req := &zondpbv1.AttestationsPoolRequest{
			Slot: &slot,
		}
		resp, err := s.ListPoolAttestations(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, 3, len(resp.Data))
		for _, datum := range resp.Data {
			assert.DeepEqual(t, datum.Data.Slot, slot)
		}
	})

	t.Run("index request", func(t *testing.T) {
		index := primitives.CommitteeIndex(4)
		req := &zondpbv1.AttestationsPoolRequest{
			CommitteeIndex: &index,
		}
		resp, err := s.ListPoolAttestations(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, 4, len(resp.Data))
		for _, datum := range resp.Data {
			assert.DeepEqual(t, datum.Data.Index, index)
		}
	})

	t.Run("both slot + index request", func(t *testing.T) {
		slot := primitives.Slot(2)
		index := primitives.CommitteeIndex(4)
		req := &zondpbv1.AttestationsPoolRequest{
			Slot:           &slot,
			CommitteeIndex: &index,
		}
		resp, err := s.ListPoolAttestations(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, 2, len(resp.Data))
		for _, datum := range resp.Data {
			assert.DeepEqual(t, datum.Data.Index, index)
			assert.DeepEqual(t, datum.Data.Slot, slot)
		}
	})
}

func TestListPoolAttesterSlashings(t *testing.T) {
	bs, err := util.NewBeaconState()
	require.NoError(t, err)
	slashing1 := &zondpbv1alpha1.AttesterSlashing{
		Attestation_1: &zondpbv1alpha1.IndexedAttestation{
			AttestingIndices: []uint64{1, 10},
			Data: &zondpbv1alpha1.AttestationData{
				Slot:            1,
				CommitteeIndex:  1,
				BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot1"), 32),
				Source: &zondpbv1alpha1.Checkpoint{
					Epoch: 1,
					Root:  bytesutil.PadTo([]byte("sourceroot1"), 32),
				},
				Target: &zondpbv1alpha1.Checkpoint{
					Epoch: 10,
					Root:  bytesutil.PadTo([]byte("targetroot1"), 32),
				},
			},
			Signatures: [][]byte{bytesutil.PadTo([]byte("signature1"), 4595)},
		},
		Attestation_2: &zondpbv1alpha1.IndexedAttestation{
			AttestingIndices: []uint64{2, 20},
			Data: &zondpbv1alpha1.AttestationData{
				Slot:            2,
				CommitteeIndex:  2,
				BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot2"), 32),
				Source: &zondpbv1alpha1.Checkpoint{
					Epoch: 2,
					Root:  bytesutil.PadTo([]byte("sourceroot2"), 32),
				},
				Target: &zondpbv1alpha1.Checkpoint{
					Epoch: 20,
					Root:  bytesutil.PadTo([]byte("targetroot2"), 32),
				},
			},
			Signatures: [][]byte{bytesutil.PadTo([]byte("signature2"), 4595)},
		},
	}
	slashing2 := &zondpbv1alpha1.AttesterSlashing{
		Attestation_1: &zondpbv1alpha1.IndexedAttestation{
			AttestingIndices: []uint64{3, 30},
			Data: &zondpbv1alpha1.AttestationData{
				Slot:            3,
				CommitteeIndex:  3,
				BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot3"), 32),
				Source: &zondpbv1alpha1.Checkpoint{
					Epoch: 3,
					Root:  bytesutil.PadTo([]byte("sourceroot3"), 32),
				},
				Target: &zondpbv1alpha1.Checkpoint{
					Epoch: 30,
					Root:  bytesutil.PadTo([]byte("targetroot3"), 32),
				},
			},
			Signatures: [][]byte{bytesutil.PadTo([]byte("signature3"), 4595)},
		},
		Attestation_2: &zondpbv1alpha1.IndexedAttestation{
			AttestingIndices: []uint64{4, 40},
			Data: &zondpbv1alpha1.AttestationData{
				Slot:            4,
				CommitteeIndex:  4,
				BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot4"), 32),
				Source: &zondpbv1alpha1.Checkpoint{
					Epoch: 4,
					Root:  bytesutil.PadTo([]byte("sourceroot4"), 32),
				},
				Target: &zondpbv1alpha1.Checkpoint{
					Epoch: 40,
					Root:  bytesutil.PadTo([]byte("targetroot4"), 32),
				},
			},
			Signatures: [][]byte{bytesutil.PadTo([]byte("signature4"), 4595)},
		},
	}

	s := &Server{
		ChainInfoFetcher: &blockchainmock.ChainService{State: bs},
		SlashingsPool:    &slashingsmock.PoolMock{PendingAttSlashings: []*zondpbv1alpha1.AttesterSlashing{slashing1, slashing2}},
	}

	resp, err := s.ListPoolAttesterSlashings(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Equal(t, 2, len(resp.Data))
	assert.DeepEqual(t, migration.V1Alpha1ToV1AttSlashing(slashing1), resp.Data[0])
	assert.DeepEqual(t, migration.V1Alpha1ToV1AttSlashing(slashing2), resp.Data[1])
}

func TestListPoolProposerSlashings(t *testing.T) {
	bs, err := util.NewBeaconState()
	require.NoError(t, err)
	slashing1 := &zondpbv1alpha1.ProposerSlashing{
		Header_1: &zondpbv1alpha1.SignedBeaconBlockHeader{
			Header: &zondpbv1alpha1.BeaconBlockHeader{
				Slot:          1,
				ProposerIndex: 1,
				ParentRoot:    bytesutil.PadTo([]byte("parentroot1"), 32),
				StateRoot:     bytesutil.PadTo([]byte("stateroot1"), 32),
				BodyRoot:      bytesutil.PadTo([]byte("bodyroot1"), 32),
			},
			Signature: bytesutil.PadTo([]byte("signature1"), 4595),
		},
		Header_2: &zondpbv1alpha1.SignedBeaconBlockHeader{
			Header: &zondpbv1alpha1.BeaconBlockHeader{
				Slot:          2,
				ProposerIndex: 2,
				ParentRoot:    bytesutil.PadTo([]byte("parentroot2"), 32),
				StateRoot:     bytesutil.PadTo([]byte("stateroot2"), 32),
				BodyRoot:      bytesutil.PadTo([]byte("bodyroot2"), 32),
			},
			Signature: bytesutil.PadTo([]byte("signature2"), 4595),
		},
	}
	slashing2 := &zondpbv1alpha1.ProposerSlashing{
		Header_1: &zondpbv1alpha1.SignedBeaconBlockHeader{
			Header: &zondpbv1alpha1.BeaconBlockHeader{
				Slot:          3,
				ProposerIndex: 3,
				ParentRoot:    bytesutil.PadTo([]byte("parentroot3"), 32),
				StateRoot:     bytesutil.PadTo([]byte("stateroot3"), 32),
				BodyRoot:      bytesutil.PadTo([]byte("bodyroot3"), 32),
			},
			Signature: bytesutil.PadTo([]byte("signature3"), 4595),
		},
		Header_2: &zondpbv1alpha1.SignedBeaconBlockHeader{
			Header: &zondpbv1alpha1.BeaconBlockHeader{
				Slot:          4,
				ProposerIndex: 4,
				ParentRoot:    bytesutil.PadTo([]byte("parentroot4"), 32),
				StateRoot:     bytesutil.PadTo([]byte("stateroot4"), 32),
				BodyRoot:      bytesutil.PadTo([]byte("bodyroot4"), 32),
			},
			Signature: bytesutil.PadTo([]byte("signature4"), 4595),
		},
	}

	s := &Server{
		ChainInfoFetcher: &blockchainmock.ChainService{State: bs},
		SlashingsPool:    &slashingsmock.PoolMock{PendingPropSlashings: []*zondpbv1alpha1.ProposerSlashing{slashing1, slashing2}},
	}

	resp, err := s.ListPoolProposerSlashings(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Equal(t, 2, len(resp.Data))
	assert.DeepEqual(t, migration.V1Alpha1ToV1ProposerSlashing(slashing1), resp.Data[0])
	assert.DeepEqual(t, migration.V1Alpha1ToV1ProposerSlashing(slashing2), resp.Data[1])
}

func TestListPoolVoluntaryExits(t *testing.T) {
	bs, err := util.NewBeaconState()
	require.NoError(t, err)
	exit1 := &zondpbv1alpha1.SignedVoluntaryExit{
		Exit: &zondpbv1alpha1.VoluntaryExit{
			Epoch:          1,
			ValidatorIndex: 1,
		},
		Signature: bytesutil.PadTo([]byte("signature1"), 4595),
	}
	exit2 := &zondpbv1alpha1.SignedVoluntaryExit{
		Exit: &zondpbv1alpha1.VoluntaryExit{
			Epoch:          2,
			ValidatorIndex: 2,
		},
		Signature: bytesutil.PadTo([]byte("signature2"), 4595),
	}

	s := &Server{
		ChainInfoFetcher:   &blockchainmock.ChainService{State: bs},
		VoluntaryExitsPool: &mock.PoolMock{Exits: []*zondpbv1alpha1.SignedVoluntaryExit{exit1, exit2}},
	}

	resp, err := s.ListPoolVoluntaryExits(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Equal(t, 2, len(resp.Data))
	assert.DeepEqual(t, migration.V1Alpha1ToV1Exit(exit1), resp.Data[0])
	assert.DeepEqual(t, migration.V1Alpha1ToV1Exit(exit2), resp.Data[1])
}

func TestSubmitAttesterSlashing_Ok(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	_, keys, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	validator := &zondpbv1alpha1.Validator{
		PublicKey: keys[0].PublicKey().Marshal(),
	}
	bs, err := util.NewBeaconState(func(state *zondpbv1alpha1.BeaconState) error {
		state.Validators = []*zondpbv1alpha1.Validator{validator}
		return nil
	})
	require.NoError(t, err)

	slashing := &zondpbv1.AttesterSlashing{
		Attestation_1: &zondpbv1.IndexedAttestation{
			AttestingIndices: []uint64{0},
			Data: &zondpbv1.AttestationData{
				Slot:            1,
				Index:           1,
				BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot1"), 32),
				Source: &zondpbv1.Checkpoint{
					Epoch: 1,
					Root:  bytesutil.PadTo([]byte("sourceroot1"), 32),
				},
				Target: &zondpbv1.Checkpoint{
					Epoch: 10,
					Root:  bytesutil.PadTo([]byte("targetroot1"), 32),
				},
			},
			Signatures: [][]byte{},
		},
		Attestation_2: &zondpbv1.IndexedAttestation{
			AttestingIndices: []uint64{0},
			Data: &zondpbv1.AttestationData{
				Slot:            1,
				Index:           1,
				BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot2"), 32),
				Source: &zondpbv1.Checkpoint{
					Epoch: 1,
					Root:  bytesutil.PadTo([]byte("sourceroot2"), 32),
				},
				Target: &zondpbv1.Checkpoint{
					Epoch: 10,
					Root:  bytesutil.PadTo([]byte("targetroot2"), 32),
				},
			},
			Signatures: [][]byte{},
		},
	}

	for _, att := range []*zondpbv1.IndexedAttestation{slashing.Attestation_1, slashing.Attestation_2} {
		sb, err := signing.ComputeDomainAndSign(bs, att.Data.Target.Epoch, att.Data, params.BeaconConfig().DomainBeaconAttester, keys[0])
		require.NoError(t, err)
		sig, err := dilithium.SignatureFromBytes(sb)
		require.NoError(t, err)
		att.Signatures = [][]byte{sig.Marshal()}
	}

	broadcaster := &p2pMock.MockBroadcaster{}
	s := &Server{
		ChainInfoFetcher: &blockchainmock.ChainService{State: bs},
		SlashingsPool:    &slashingsmock.PoolMock{},
		Broadcaster:      broadcaster,
	}

	_, err = s.SubmitAttesterSlashing(ctx, slashing)
	require.NoError(t, err)
	pendingSlashings := s.SlashingsPool.PendingAttesterSlashings(ctx, bs, true)
	require.Equal(t, 1, len(pendingSlashings))
	assert.DeepEqual(t, migration.V1ToV1Alpha1AttSlashing(slashing), pendingSlashings[0])
	assert.Equal(t, true, broadcaster.BroadcastCalled)
	require.Equal(t, 1, len(broadcaster.BroadcastMessages))
	_, ok := broadcaster.BroadcastMessages[0].(*zondpbv1alpha1.AttesterSlashing)
	assert.Equal(t, true, ok)
}

// TODO (rgeraldes24) double check
func TestSubmitAttesterSlashing_AcrossFork(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	params.SetupTestConfigCleanup(t)
	// config := params.BeaconConfig()
	// config.AltairForkEpoch = 1
	// params.OverrideBeaconConfig(config)

	bs, keys := util.DeterministicGenesisState(t, 1)

	slashing := &zondpbv1.AttesterSlashing{
		Attestation_1: &zondpbv1.IndexedAttestation{
			AttestingIndices: []uint64{0},
			Data: &zondpbv1.AttestationData{
				Slot:            params.BeaconConfig().SlotsPerEpoch,
				Index:           1,
				BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot1"), 32),
				Source: &zondpbv1.Checkpoint{
					Epoch: 1,
					Root:  bytesutil.PadTo([]byte("sourceroot1"), 32),
				},
				Target: &zondpbv1.Checkpoint{
					Epoch: 10,
					Root:  bytesutil.PadTo([]byte("targetroot1"), 32),
				},
			},
			Signatures: [][]byte{},
		},
		Attestation_2: &zondpbv1.IndexedAttestation{
			AttestingIndices: []uint64{0},
			Data: &zondpbv1.AttestationData{
				Slot:            params.BeaconConfig().SlotsPerEpoch,
				Index:           1,
				BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot2"), 32),
				Source: &zondpbv1.Checkpoint{
					Epoch: 1,
					Root:  bytesutil.PadTo([]byte("sourceroot2"), 32),
				},
				Target: &zondpbv1.Checkpoint{
					Epoch: 10,
					Root:  bytesutil.PadTo([]byte("targetroot2"), 32),
				},
			},
			Signatures: [][]byte{},
		},
	}

	newBs := bs.Copy()
	newBs, err := transition.ProcessSlots(ctx, newBs, params.BeaconConfig().SlotsPerEpoch)
	require.NoError(t, err)

	for _, att := range []*zondpbv1.IndexedAttestation{slashing.Attestation_1, slashing.Attestation_2} {
		sb, err := signing.ComputeDomainAndSign(newBs, att.Data.Target.Epoch, att.Data, params.BeaconConfig().DomainBeaconAttester, keys[0])
		require.NoError(t, err)
		sig, err := dilithium.SignatureFromBytes(sb)
		require.NoError(t, err)
		att.Signatures = [][]byte{sig.Marshal()}
	}

	broadcaster := &p2pMock.MockBroadcaster{}
	s := &Server{
		ChainInfoFetcher: &blockchainmock.ChainService{State: bs},
		SlashingsPool:    &slashingsmock.PoolMock{},
		Broadcaster:      broadcaster,
	}

	_, err = s.SubmitAttesterSlashing(ctx, slashing)
	require.NoError(t, err)
	pendingSlashings := s.SlashingsPool.PendingAttesterSlashings(ctx, bs, true)
	require.Equal(t, 1, len(pendingSlashings))
	assert.DeepEqual(t, migration.V1ToV1Alpha1AttSlashing(slashing), pendingSlashings[0])
	assert.Equal(t, true, broadcaster.BroadcastCalled)
	require.Equal(t, 1, len(broadcaster.BroadcastMessages))
	_, ok := broadcaster.BroadcastMessages[0].(*zondpbv1alpha1.AttesterSlashing)
	assert.Equal(t, true, ok)
}

func TestSubmitAttesterSlashing_InvalidSlashing(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	bs, err := util.NewBeaconState()
	require.NoError(t, err)

	attestation := &zondpbv1.IndexedAttestation{
		AttestingIndices: []uint64{0},
		Data: &zondpbv1.AttestationData{
			Slot:            1,
			Index:           1,
			BeaconBlockRoot: bytesutil.PadTo([]byte("blockroot1"), 32),
			Source: &zondpbv1.Checkpoint{
				Epoch: 1,
				Root:  bytesutil.PadTo([]byte("sourceroot1"), 32),
			},
			Target: &zondpbv1.Checkpoint{
				Epoch: 10,
				Root:  bytesutil.PadTo([]byte("targetroot1"), 32),
			},
		},
		Signatures: [][]byte{},
	}

	slashing := &zondpbv1.AttesterSlashing{
		Attestation_1: attestation,
		Attestation_2: attestation,
	}

	broadcaster := &p2pMock.MockBroadcaster{}
	s := &Server{
		ChainInfoFetcher: &blockchainmock.ChainService{State: bs},
		SlashingsPool:    &slashingsmock.PoolMock{},
		Broadcaster:      broadcaster,
	}

	_, err = s.SubmitAttesterSlashing(ctx, slashing)
	require.ErrorContains(t, "Invalid attester slashing", err)
	assert.Equal(t, false, broadcaster.BroadcastCalled)
}

func TestSubmitProposerSlashing_Ok(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	_, keys, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	validator := &zondpbv1alpha1.Validator{
		PublicKey:         keys[0].PublicKey().Marshal(),
		WithdrawableEpoch: primitives.Epoch(1),
	}
	bs, err := util.NewBeaconState(func(state *zondpbv1alpha1.BeaconState) error {
		state.Validators = []*zondpbv1alpha1.Validator{validator}
		return nil
	})
	require.NoError(t, err)

	slashing := &zondpbv1.ProposerSlashing{
		SignedHeader_1: &zondpbv1.SignedBeaconBlockHeader{
			Message: &zondpbv1.BeaconBlockHeader{
				Slot:          1,
				ProposerIndex: 0,
				ParentRoot:    bytesutil.PadTo([]byte("parentroot1"), 32),
				StateRoot:     bytesutil.PadTo([]byte("stateroot1"), 32),
				BodyRoot:      bytesutil.PadTo([]byte("bodyroot1"), 32),
			},
			Signature: make([]byte, 4595),
		},
		SignedHeader_2: &zondpbv1.SignedBeaconBlockHeader{
			Message: &zondpbv1.BeaconBlockHeader{
				Slot:          1,
				ProposerIndex: 0,
				ParentRoot:    bytesutil.PadTo([]byte("parentroot2"), 32),
				StateRoot:     bytesutil.PadTo([]byte("stateroot2"), 32),
				BodyRoot:      bytesutil.PadTo([]byte("bodyroot2"), 32),
			},
			Signature: make([]byte, 4595),
		},
	}

	for _, h := range []*zondpbv1.SignedBeaconBlockHeader{slashing.SignedHeader_1, slashing.SignedHeader_2} {
		sb, err := signing.ComputeDomainAndSign(
			bs,
			slots.ToEpoch(h.Message.Slot),
			h.Message,
			params.BeaconConfig().DomainBeaconProposer,
			keys[0],
		)
		require.NoError(t, err)
		sig, err := dilithium.SignatureFromBytes(sb)
		require.NoError(t, err)
		h.Signature = sig.Marshal()
	}

	broadcaster := &p2pMock.MockBroadcaster{}
	s := &Server{
		ChainInfoFetcher: &blockchainmock.ChainService{State: bs},
		SlashingsPool:    &slashingsmock.PoolMock{},
		Broadcaster:      broadcaster,
	}

	_, err = s.SubmitProposerSlashing(ctx, slashing)
	require.NoError(t, err)
	pendingSlashings := s.SlashingsPool.PendingProposerSlashings(ctx, bs, true)
	require.Equal(t, 1, len(pendingSlashings))
	assert.DeepEqual(t, migration.V1ToV1Alpha1ProposerSlashing(slashing), pendingSlashings[0])
	assert.Equal(t, true, broadcaster.BroadcastCalled)
	require.Equal(t, 1, len(broadcaster.BroadcastMessages))
	_, ok := broadcaster.BroadcastMessages[0].(*zondpbv1alpha1.ProposerSlashing)
	assert.Equal(t, true, ok)
}

func TestSubmitProposerSlashing_AcrossFork(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	// params.SetupTestConfigCleanup(t)
	// config := params.BeaconConfig()
	// config.AltairForkEpoch = 1
	// params.OverrideBeaconConfig(config)

	bs, keys := util.DeterministicGenesisState(t, 1)

	slashing := &zondpbv1.ProposerSlashing{
		SignedHeader_1: &zondpbv1.SignedBeaconBlockHeader{
			Message: &zondpbv1.BeaconBlockHeader{
				Slot:          params.BeaconConfig().SlotsPerEpoch,
				ProposerIndex: 0,
				ParentRoot:    bytesutil.PadTo([]byte("parentroot1"), 32),
				StateRoot:     bytesutil.PadTo([]byte("stateroot1"), 32),
				BodyRoot:      bytesutil.PadTo([]byte("bodyroot1"), 32),
			},
			Signature: make([]byte, 4595),
		},
		SignedHeader_2: &zondpbv1.SignedBeaconBlockHeader{
			Message: &zondpbv1.BeaconBlockHeader{
				Slot:          params.BeaconConfig().SlotsPerEpoch,
				ProposerIndex: 0,
				ParentRoot:    bytesutil.PadTo([]byte("parentroot2"), 32),
				StateRoot:     bytesutil.PadTo([]byte("stateroot2"), 32),
				BodyRoot:      bytesutil.PadTo([]byte("bodyroot2"), 32),
			},
			Signature: make([]byte, 4595),
		},
	}

	newBs := bs.Copy()
	newBs, err := transition.ProcessSlots(ctx, newBs, params.BeaconConfig().SlotsPerEpoch)
	require.NoError(t, err)

	for _, h := range []*zondpbv1.SignedBeaconBlockHeader{slashing.SignedHeader_1, slashing.SignedHeader_2} {
		sb, err := signing.ComputeDomainAndSign(
			newBs,
			slots.ToEpoch(h.Message.Slot),
			h.Message,
			params.BeaconConfig().DomainBeaconProposer,
			keys[0],
		)
		require.NoError(t, err)
		sig, err := dilithium.SignatureFromBytes(sb)
		require.NoError(t, err)
		h.Signature = sig.Marshal()
	}

	broadcaster := &p2pMock.MockBroadcaster{}
	s := &Server{
		ChainInfoFetcher: &blockchainmock.ChainService{State: bs},
		SlashingsPool:    &slashingsmock.PoolMock{},
		Broadcaster:      broadcaster,
	}

	_, err = s.SubmitProposerSlashing(ctx, slashing)
	require.NoError(t, err)
	pendingSlashings := s.SlashingsPool.PendingProposerSlashings(ctx, bs, true)
	require.Equal(t, 1, len(pendingSlashings))
	assert.DeepEqual(t, migration.V1ToV1Alpha1ProposerSlashing(slashing), pendingSlashings[0])
	assert.Equal(t, true, broadcaster.BroadcastCalled)
	require.Equal(t, 1, len(broadcaster.BroadcastMessages))
	_, ok := broadcaster.BroadcastMessages[0].(*zondpbv1alpha1.ProposerSlashing)
	assert.Equal(t, true, ok)
}

func TestSubmitProposerSlashing_InvalidSlashing(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	bs, err := util.NewBeaconState()
	require.NoError(t, err)

	header := &zondpbv1.SignedBeaconBlockHeader{
		Message: &zondpbv1.BeaconBlockHeader{
			Slot:          1,
			ProposerIndex: 0,
			ParentRoot:    bytesutil.PadTo([]byte("parentroot1"), 32),
			StateRoot:     bytesutil.PadTo([]byte("stateroot1"), 32),
			BodyRoot:      bytesutil.PadTo([]byte("bodyroot1"), 32),
		},
		Signature: make([]byte, 4595),
	}

	slashing := &zondpbv1.ProposerSlashing{
		SignedHeader_1: header,
		SignedHeader_2: header,
	}

	broadcaster := &p2pMock.MockBroadcaster{}
	s := &Server{
		ChainInfoFetcher: &blockchainmock.ChainService{State: bs},
		SlashingsPool:    &slashingsmock.PoolMock{},
		Broadcaster:      broadcaster,
	}

	_, err = s.SubmitProposerSlashing(ctx, slashing)
	require.ErrorContains(t, "Invalid proposer slashing", err)
	assert.Equal(t, false, broadcaster.BroadcastCalled)
}

func TestSubmitVoluntaryExit_Ok(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	_, keys, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	validator := &zondpbv1alpha1.Validator{
		ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		PublicKey: keys[0].PublicKey().Marshal(),
	}
	bs, err := util.NewBeaconState(func(state *zondpbv1alpha1.BeaconState) error {
		state.Validators = []*zondpbv1alpha1.Validator{validator}
		// Satisfy activity time required before exiting.
		state.Slot = params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().ShardCommitteePeriod))
		return nil
	})
	require.NoError(t, err)

	exit := &zondpbv1.SignedVoluntaryExit{
		Message: &zondpbv1.VoluntaryExit{
			Epoch:          0,
			ValidatorIndex: 0,
		},
		Signature: make([]byte, 4595),
	}

	sb, err := signing.ComputeDomainAndSign(bs, exit.Message.Epoch, exit.Message, params.BeaconConfig().DomainVoluntaryExit, keys[0])
	require.NoError(t, err)
	sig, err := dilithium.SignatureFromBytes(sb)
	require.NoError(t, err)
	exit.Signature = sig.Marshal()

	broadcaster := &p2pMock.MockBroadcaster{}
	s := &Server{
		ChainInfoFetcher:   &blockchainmock.ChainService{State: bs},
		VoluntaryExitsPool: &mock.PoolMock{},
		Broadcaster:        broadcaster,
	}

	_, err = s.SubmitVoluntaryExit(ctx, exit)
	require.NoError(t, err)
	pendingExits, err := s.VoluntaryExitsPool.PendingExits()
	require.NoError(t, err)
	require.Equal(t, 1, len(pendingExits))
	assert.DeepEqual(t, migration.V1ToV1Alpha1Exit(exit), pendingExits[0])
	assert.Equal(t, true, broadcaster.BroadcastCalled)
}

func TestSubmitVoluntaryExit_AcrossFork(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	bs, keys := util.DeterministicGenesisState(t, 1)
	// Satisfy activity time required before exiting.
	require.NoError(t, bs.SetSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().ShardCommitteePeriod))))

	exit := &zondpbv1.SignedVoluntaryExit{
		Message: &zondpbv1.VoluntaryExit{
			Epoch:          params.BeaconConfig().ShardCommitteePeriod + 1,
			ValidatorIndex: 0,
		},
		Signature: make([]byte, 4595),
	}

	newBs := bs.Copy()
	newBs, err := transition.ProcessSlots(ctx, newBs, params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().ShardCommitteePeriod)+1))
	require.NoError(t, err)

	sb, err := signing.ComputeDomainAndSign(newBs, exit.Message.Epoch, exit.Message, params.BeaconConfig().DomainVoluntaryExit, keys[0])
	require.NoError(t, err)
	sig, err := dilithium.SignatureFromBytes(sb)
	require.NoError(t, err)
	exit.Signature = sig.Marshal()

	broadcaster := &p2pMock.MockBroadcaster{}
	s := &Server{
		ChainInfoFetcher:   &blockchainmock.ChainService{State: bs},
		VoluntaryExitsPool: &mock.PoolMock{},
		Broadcaster:        broadcaster,
	}

	_, err = s.SubmitVoluntaryExit(ctx, exit)
	require.NoError(t, err)
}

func TestSubmitVoluntaryExit_InvalidValidatorIndex(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	_, keys, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	validator := &zondpbv1alpha1.Validator{
		ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		PublicKey: keys[0].PublicKey().Marshal(),
	}
	bs, err := util.NewBeaconState(func(state *zondpbv1alpha1.BeaconState) error {
		state.Validators = []*zondpbv1alpha1.Validator{validator}
		return nil
	})
	require.NoError(t, err)

	exit := &zondpbv1.SignedVoluntaryExit{
		Message: &zondpbv1.VoluntaryExit{
			Epoch:          0,
			ValidatorIndex: 99,
		},
		Signature: make([]byte, 4595),
	}

	broadcaster := &p2pMock.MockBroadcaster{}
	s := &Server{
		ChainInfoFetcher:   &blockchainmock.ChainService{State: bs},
		VoluntaryExitsPool: &mock.PoolMock{},
		Broadcaster:        broadcaster,
	}

	_, err = s.SubmitVoluntaryExit(ctx, exit)
	require.ErrorContains(t, "Could not get exiting validator", err)
	assert.Equal(t, false, broadcaster.BroadcastCalled)
}

func TestSubmitVoluntaryExit_InvalidExit(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	_, keys, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	validator := &zondpbv1alpha1.Validator{
		ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		PublicKey: keys[0].PublicKey().Marshal(),
	}
	bs, err := util.NewBeaconState(func(state *zondpbv1alpha1.BeaconState) error {
		state.Validators = []*zondpbv1alpha1.Validator{validator}
		return nil
	})
	require.NoError(t, err)

	exit := &zondpbv1.SignedVoluntaryExit{
		Message: &zondpbv1.VoluntaryExit{
			Epoch:          0,
			ValidatorIndex: 0,
		},
		Signature: make([]byte, 4595),
	}

	broadcaster := &p2pMock.MockBroadcaster{}
	s := &Server{
		ChainInfoFetcher:   &blockchainmock.ChainService{State: bs},
		VoluntaryExitsPool: &mock.PoolMock{},
		Broadcaster:        broadcaster,
	}

	_, err = s.SubmitVoluntaryExit(ctx, exit)
	require.ErrorContains(t, "Invalid voluntary exit", err)
	assert.Equal(t, false, broadcaster.BroadcastCalled)
}

func TestServer_SubmitAttestations_Ok(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	params.SetupTestConfigCleanup(t)
	c := params.BeaconConfig().Copy()
	// Required for correct committee size calculation.
	c.SlotsPerEpoch = 1
	params.OverrideBeaconConfig(c)

	_, keys, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	validators := []*zondpbv1alpha1.Validator{
		{
			PublicKey: keys[0].PublicKey().Marshal(),
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		},
	}
	bs, err := util.NewBeaconState(func(state *zondpbv1alpha1.BeaconState) error {
		state.Validators = validators
		state.Slot = 1
		state.PreviousJustifiedCheckpoint = &zondpbv1alpha1.Checkpoint{
			Epoch: 0,
			Root:  bytesutil.PadTo([]byte("sourceroot1"), 32),
		}
		return nil
	})
	require.NoError(t, err)
	b := bitfield.NewBitlist(1)
	b.SetBitAt(0, true)

	sourceCheckpoint := &zondpbv1.Checkpoint{
		Epoch: 0,
		Root:  bytesutil.PadTo([]byte("sourceroot1"), 32),
	}
	att1 := &zondpbv1.Attestation{
		ParticipationBits: b,
		Data: &zondpbv1.AttestationData{
			Slot:            0,
			Index:           0,
			BeaconBlockRoot: bytesutil.PadTo([]byte("beaconblockroot1"), 32),
			Source:          sourceCheckpoint,
			Target: &zondpbv1.Checkpoint{
				Epoch: 0,
				Root:  bytesutil.PadTo([]byte("targetroot1"), 32),
			},
		},
		Signatures: [][]byte{},
	}
	att2 := &zondpbv1.Attestation{
		ParticipationBits: b,
		Data: &zondpbv1.AttestationData{
			Slot:            0,
			Index:           0,
			BeaconBlockRoot: bytesutil.PadTo([]byte("beaconblockroot2"), 32),
			Source:          sourceCheckpoint,
			Target: &zondpbv1.Checkpoint{
				Epoch: 0,
				Root:  bytesutil.PadTo([]byte("targetroot2"), 32),
			},
		},
		Signatures: [][]byte{},
	}

	for _, att := range []*zondpbv1.Attestation{att1, att2} {
		sb, err := signing.ComputeDomainAndSign(
			bs,
			slots.ToEpoch(att.Data.Slot),
			att.Data,
			params.BeaconConfig().DomainBeaconAttester,
			keys[0],
		)
		require.NoError(t, err)
		sig, err := dilithium.SignatureFromBytes(sb)
		require.NoError(t, err)
		att.Signatures = [][]byte{sig.Marshal()}
	}

	broadcaster := &p2pMock.MockBroadcaster{}
	chainService := &blockchainmock.ChainService{State: bs}
	s := &Server{
		HeadFetcher:       chainService,
		ChainInfoFetcher:  chainService,
		AttestationsPool:  attestations.NewPool(),
		Broadcaster:       broadcaster,
		OperationNotifier: &blockchainmock.MockOperationNotifier{},
	}

	_, err = s.SubmitAttestations(ctx, &zondpbv1.SubmitAttestationsRequest{
		Data: []*zondpbv1.Attestation{att1, att2},
	})
	require.NoError(t, err)
	assert.Equal(t, true, broadcaster.BroadcastCalled)
	assert.Equal(t, 2, len(broadcaster.BroadcastAttestations))
	expectedAtt1, err := att1.HashTreeRoot()
	require.NoError(t, err)
	expectedAtt2, err := att2.HashTreeRoot()
	require.NoError(t, err)
	actualAtt1, err := broadcaster.BroadcastAttestations[0].HashTreeRoot()
	require.NoError(t, err)
	actualAtt2, err := broadcaster.BroadcastAttestations[1].HashTreeRoot()
	require.NoError(t, err)
	for _, r := range [][32]byte{actualAtt1, actualAtt2} {
		assert.Equal(t, true, reflect.DeepEqual(expectedAtt1, r) || reflect.DeepEqual(expectedAtt2, r))
	}

	require.Equal(t, 2, s.AttestationsPool.UnaggregatedAttestationCount())
}

func TestServer_SubmitAttestations_ValidAttestationSubmitted(t *testing.T) {
	ctx := grpc.NewContextWithServerTransportStream(context.Background(), &runtime.ServerTransportStream{})

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	params.SetupTestConfigCleanup(t)
	c := params.BeaconConfig().Copy()
	// Required for correct committee size calculation.
	c.SlotsPerEpoch = 1
	params.OverrideBeaconConfig(c)

	_, keys, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	validators := []*zondpbv1alpha1.Validator{
		{
			PublicKey: keys[0].PublicKey().Marshal(),
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		},
	}
	bs, err := util.NewBeaconState(func(state *zondpbv1alpha1.BeaconState) error {
		state.Validators = validators
		state.Slot = 1
		state.PreviousJustifiedCheckpoint = &zondpbv1alpha1.Checkpoint{
			Epoch: 0,
			Root:  bytesutil.PadTo([]byte("sourceroot1"), 32),
		}
		return nil
	})

	require.NoError(t, err)

	sourceCheckpoint := &zondpbv1.Checkpoint{
		Epoch: 0,
		Root:  bytesutil.PadTo([]byte("sourceroot1"), 32),
	}
	b := bitfield.NewBitlist(1)
	b.SetBitAt(0, true)
	attValid := &zondpbv1.Attestation{
		ParticipationBits: b,
		Data: &zondpbv1.AttestationData{
			Slot:            0,
			Index:           0,
			BeaconBlockRoot: bytesutil.PadTo([]byte("beaconblockroot1"), 32),
			Source:          sourceCheckpoint,
			Target: &zondpbv1.Checkpoint{
				Epoch: 0,
				Root:  bytesutil.PadTo([]byte("targetroot1"), 32),
			},
		},
		Signatures: [][]byte{},
	}
	attInvalidSignature := &zondpbv1.Attestation{
		ParticipationBits: b,
		Data: &zondpbv1.AttestationData{
			Slot:            0,
			Index:           0,
			BeaconBlockRoot: bytesutil.PadTo([]byte("beaconblockroot2"), 32),
			Source:          sourceCheckpoint,
			Target: &zondpbv1.Checkpoint{
				Epoch: 0,
				Root:  bytesutil.PadTo([]byte("targetroot2"), 32),
			},
		},
		Signatures: [][]byte{[]byte{}},
	}

	// Don't sign attInvalidSignature.
	sb, err := signing.ComputeDomainAndSign(
		bs,
		slots.ToEpoch(attValid.Data.Slot),
		attValid.Data,
		params.BeaconConfig().DomainBeaconAttester,
		keys[0],
	)
	require.NoError(t, err)
	sig, err := dilithium.SignatureFromBytes(sb)
	require.NoError(t, err)
	attValid.Signatures = [][]byte{sig.Marshal()}

	broadcaster := &p2pMock.MockBroadcaster{}
	chainService := &blockchainmock.ChainService{State: bs}
	s := &Server{
		HeadFetcher:       chainService,
		ChainInfoFetcher:  chainService,
		AttestationsPool:  attestations.NewPool(),
		Broadcaster:       broadcaster,
		OperationNotifier: &blockchainmock.MockOperationNotifier{},
	}

	_, err = s.SubmitAttestations(ctx, &zondpbv1.SubmitAttestationsRequest{
		Data: []*zondpbv1.Attestation{attValid, attInvalidSignature},
	})
	require.ErrorContains(t, "One or more attestations failed validation", err)
	expectedAtt, err := attValid.HashTreeRoot()
	require.NoError(t, err)
	assert.Equal(t, true, broadcaster.BroadcastCalled)
	require.Equal(t, 1, len(broadcaster.BroadcastAttestations))
	broadcastRoot, err := broadcaster.BroadcastAttestations[0].HashTreeRoot()
	require.NoError(t, err)
	require.DeepEqual(t, expectedAtt, broadcastRoot)

	require.Equal(t, 1, s.AttestationsPool.UnaggregatedAttestationCount())
}

func TestServer_SubmitAttestations_InvalidAttestationGRPCHeader(t *testing.T) {
	ctx := grpc.NewContextWithServerTransportStream(context.Background(), &runtime.ServerTransportStream{})

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	params.SetupTestConfigCleanup(t)
	c := params.BeaconConfig().Copy()
	// Required for correct committee size calculation.
	c.SlotsPerEpoch = 1
	params.OverrideBeaconConfig(c)

	_, keys, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	validators := []*zondpbv1alpha1.Validator{
		{
			PublicKey: keys[0].PublicKey().Marshal(),
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		},
	}
	bs, err := util.NewBeaconState(func(state *zondpbv1alpha1.BeaconState) error {
		state.Validators = validators
		state.Slot = 1
		state.PreviousJustifiedCheckpoint = &zondpbv1alpha1.Checkpoint{
			Epoch: 0,
			Root:  bytesutil.PadTo([]byte("sourceroot1"), 32),
		}
		return nil
	})

	require.NoError(t, err)

	b := bitfield.NewBitlist(1)
	b.SetBitAt(0, true)
	att := &zondpbv1.Attestation{
		ParticipationBits: b,
		Data: &zondpbv1.AttestationData{
			Slot:            0,
			Index:           0,
			BeaconBlockRoot: bytesutil.PadTo([]byte("beaconblockroot2"), 32),
			Source: &zondpbv1.Checkpoint{
				Epoch: 0,
				Root:  bytesutil.PadTo([]byte("sourceroot2"), 32),
			},
			Target: &zondpbv1.Checkpoint{
				Epoch: 1,
				Root:  bytesutil.PadTo([]byte("targetroot2"), 32),
			},
		},
		Signatures: [][]byte{{}},
	}

	chain := &blockchainmock.ChainService{State: bs}
	broadcaster := &p2pMock.MockBroadcaster{}
	s := &Server{
		ChainInfoFetcher:  chain,
		AttestationsPool:  attestations.NewPool(),
		Broadcaster:       broadcaster,
		OperationNotifier: &blockchainmock.MockOperationNotifier{},
		HeadFetcher:       chain,
	}

	_, err = s.SubmitAttestations(ctx, &zondpbv1.SubmitAttestationsRequest{
		Data: []*zondpbv1.Attestation{att},
	})
	require.ErrorContains(t, "One or more attestations failed validation", err)
	sts, ok := grpc.ServerTransportStreamFromContext(ctx).(*runtime.ServerTransportStream)
	require.Equal(t, true, ok, "type assertion failed")
	md := sts.Header()
	v, ok := md[strings.ToLower(grpcutil.CustomErrorMetadataKey)]
	require.Equal(t, true, ok, "could not retrieve custom error metadata value")
	assert.DeepEqual(
		t,
		[]string{"{\"failures\":[{\"index\":0,\"message\":\"Incorrect attestation signature: signature must be 4595 bytes\"}]}"},
		v,
	)
}

func TestListDilithiumToExecutionChanges(t *testing.T) {
	change1 := &zondpbv1alpha1.SignedDilithiumToExecutionChange{
		Message: &zondpbv1alpha1.DilithiumToExecutionChange{
			ValidatorIndex:      1,
			FromDilithiumPubkey: bytesutil.PadTo([]byte("pubkey1"), 2592),
			ToExecutionAddress:  bytesutil.PadTo([]byte("address1"), 20),
		},
		Signature: bytesutil.PadTo([]byte("signature1"), 4595),
	}
	change2 := &zondpbv1alpha1.SignedDilithiumToExecutionChange{
		Message: &zondpbv1alpha1.DilithiumToExecutionChange{
			ValidatorIndex:      2,
			FromDilithiumPubkey: bytesutil.PadTo([]byte("pubkey2"), 2592),
			ToExecutionAddress:  bytesutil.PadTo([]byte("address2"), 20),
		},
		Signature: bytesutil.PadTo([]byte("signature2"), 4595),
	}

	s := &Server{
		DilithiumChangesPool: &dilithiumtoexecmock.PoolMock{Changes: []*zondpbv1alpha1.SignedDilithiumToExecutionChange{change1, change2}},
	}

	resp, err := s.ListDilithiumToExecutionChanges(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Equal(t, 2, len(resp.Data))
	assert.DeepEqual(t, migration.V1Alpha1ToV1SignedDilithiumToExecChange(change1), resp.Data[0])
	assert.DeepEqual(t, migration.V1Alpha1ToV1SignedDilithiumToExecChange(change2), resp.Data[1])
}

func TestSubmitSignedDilithiumToExecutionChanges_Ok(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	params.SetupTestConfigCleanup(t)

	spb := &zondpbv1alpha1.BeaconState{
		Fork: &zondpbv1alpha1.Fork{
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			//Epoch:           params.BeaconConfig().CapellaForkEpoch,
		},
	}
	numValidators := 10
	validators := make([]*zondpbv1alpha1.Validator, numValidators)
	dilithiumChanges := make([]*zondpbv1.DilithiumToExecutionChange, numValidators)
	spb.Balances = make([]uint64, numValidators)
	privKeys := make([]common.SecretKey, numValidators)
	maxEffectiveBalance := params.BeaconConfig().MaxEffectiveBalance
	executionAddress := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13}

	for i := range validators {
		v := &zondpbv1alpha1.Validator{}
		v.EffectiveBalance = maxEffectiveBalance
		v.WithdrawableEpoch = params.BeaconConfig().FarFutureEpoch
		v.WithdrawalCredentials = make([]byte, 32)
		priv, err := dilithium.RandKey()
		require.NoError(t, err)
		privKeys[i] = priv
		pubkey := priv.PublicKey().Marshal()

		message := &zondpbv1.DilithiumToExecutionChange{
			ToExecutionAddress:  executionAddress,
			ValidatorIndex:      primitives.ValidatorIndex(i),
			FromDilithiumPubkey: pubkey,
		}

		hashFn := ssz.NewHasherFunc(hash.CustomSHA256Hasher())
		digest := hashFn.Hash(pubkey)
		digest[0] = params.BeaconConfig().DilithiumWithdrawalPrefixByte
		copy(v.WithdrawalCredentials, digest[:])
		validators[i] = v
		dilithiumChanges[i] = message
	}
	spb.Validators = validators
	slot := primitives.Slot(0)
	spb.Slot = slot
	st, err := state_native.InitializeFromProtoCapella(spb)
	require.NoError(t, err)

	signedChanges := make([]*zondpbv1.SignedDilithiumToExecutionChange, numValidators)
	for i, message := range dilithiumChanges {
		signature, err := signing.ComputeDomainAndSign(st, qrysmtime.CurrentEpoch(st), message, params.BeaconConfig().DomainDilithiumToExecutionChange, privKeys[i])
		require.NoError(t, err)

		signed := &zondpbv1.SignedDilithiumToExecutionChange{
			Message:   message,
			Signature: signature,
		}
		signedChanges[i] = signed
	}

	broadcaster := &p2pMock.MockBroadcaster{}
	chainService := &blockchainmock.ChainService{State: st}
	s := &Server{
		HeadFetcher:          chainService,
		ChainInfoFetcher:     chainService,
		AttestationsPool:     attestations.NewPool(),
		Broadcaster:          broadcaster,
		OperationNotifier:    &blockchainmock.MockOperationNotifier{},
		DilithiumChangesPool: dilithiumtoexec.NewPool(),
	}

	_, err = s.SubmitSignedDilithiumToExecutionChanges(ctx, &zondpbv1.SubmitDilithiumToExecutionChangesRequest{
		Changes: signedChanges,
	})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond) // Delay to let the routine start
	assert.Equal(t, true, broadcaster.BroadcastCalled)
	assert.Equal(t, numValidators, len(broadcaster.BroadcastMessages))

	poolChanges, err := s.DilithiumChangesPool.PendingDilithiumToExecChanges()
	require.Equal(t, len(poolChanges), len(signedChanges))
	require.NoError(t, err)
	for i, v1alphaChange := range poolChanges {
		v1Change := migration.V1Alpha1ToV1SignedDilithiumToExecChange(v1alphaChange)
		require.DeepEqual(t, v1Change, signedChanges[i])
	}
}

func TestSubmitSignedDilithiumToExecutionChanges_Failures(t *testing.T) {
	ctx := context.Background()

	transition.SkipSlotCache.Disable()
	defer transition.SkipSlotCache.Enable()

	params.SetupTestConfigCleanup(t)

	spb := &zondpbv1alpha1.BeaconState{
		Fork: &zondpbv1alpha1.Fork{
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			//Epoch:           params.BeaconConfig().CapellaForkEpoch,
		},
	}
	numValidators := 10
	validators := make([]*zondpbv1alpha1.Validator, numValidators)
	dilithiumChanges := make([]*zondpbv1.DilithiumToExecutionChange, numValidators)
	spb.Balances = make([]uint64, numValidators)
	privKeys := make([]common.SecretKey, numValidators)
	maxEffectiveBalance := params.BeaconConfig().MaxEffectiveBalance
	executionAddress := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13}

	for i := range validators {
		v := &zondpbv1alpha1.Validator{}
		v.EffectiveBalance = maxEffectiveBalance
		v.WithdrawableEpoch = params.BeaconConfig().FarFutureEpoch
		v.WithdrawalCredentials = make([]byte, 32)
		priv, err := dilithium.RandKey()
		require.NoError(t, err)
		privKeys[i] = priv
		pubkey := priv.PublicKey().Marshal()

		message := &zondpbv1.DilithiumToExecutionChange{
			ToExecutionAddress:  executionAddress,
			ValidatorIndex:      primitives.ValidatorIndex(i),
			FromDilithiumPubkey: pubkey,
		}

		hashFn := ssz.NewHasherFunc(hash.CustomSHA256Hasher())
		digest := hashFn.Hash(pubkey)
		digest[0] = params.BeaconConfig().DilithiumWithdrawalPrefixByte
		copy(v.WithdrawalCredentials, digest[:])
		validators[i] = v
		dilithiumChanges[i] = message
	}
	spb.Validators = validators
	slot := primitives.Slot(0)
	spb.Slot = slot
	st, err := state_native.InitializeFromProtoCapella(spb)
	require.NoError(t, err)

	signedChanges := make([]*zondpbv1.SignedDilithiumToExecutionChange, numValidators)
	for i, message := range dilithiumChanges {
		signature, err := signing.ComputeDomainAndSign(st, qrysmtime.CurrentEpoch(st), message, params.BeaconConfig().DomainDilithiumToExecutionChange, privKeys[i])
		require.NoError(t, err)

		signed := &zondpbv1.SignedDilithiumToExecutionChange{
			Message:   message,
			Signature: signature,
		}
		signedChanges[i] = signed
	}
	signedChanges[1].Signature[0] = 0x00

	broadcaster := &p2pMock.MockBroadcaster{}
	chainService := &blockchainmock.ChainService{State: st}
	s := &Server{
		HeadFetcher:          chainService,
		ChainInfoFetcher:     chainService,
		AttestationsPool:     attestations.NewPool(),
		Broadcaster:          broadcaster,
		OperationNotifier:    &blockchainmock.MockOperationNotifier{},
		DilithiumChangesPool: dilithiumtoexec.NewPool(),
	}

	_, err = s.SubmitSignedDilithiumToExecutionChanges(ctx, &zondpbv1.SubmitDilithiumToExecutionChangesRequest{
		Changes: signedChanges,
	})
	time.Sleep(10 * time.Millisecond) // Delay to allow the routine to start
	require.ErrorContains(t, "One or more DilithiumToExecutionChange failed validation", err)
	assert.Equal(t, true, broadcaster.BroadcastCalled)
	assert.Equal(t, numValidators, len(broadcaster.BroadcastMessages)+1)

	poolChanges, err := s.DilithiumChangesPool.PendingDilithiumToExecChanges()
	require.Equal(t, len(poolChanges)+1, len(signedChanges))
	require.NoError(t, err)

	v1Change := migration.V1Alpha1ToV1SignedDilithiumToExecChange(poolChanges[0])
	require.DeepEqual(t, v1Change, signedChanges[0])
	for i := 2; i < numValidators; i++ {
		v1Change := migration.V1Alpha1ToV1SignedDilithiumToExecChange(poolChanges[i-1])
		require.DeepEqual(t, v1Change, signedChanges[i])
	}
}
