package state_native

import (
	"fmt"

	"github.com/pkg/errors"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/runtime/version"
)

// ToProtoUnsafe returns the pointer value of the underlying
// beacon state proto object, bypassing immutability. Use with care.
func (b *BeaconState) ToProtoUnsafe() interface{} {
	if b == nil {
		return nil
	}

	gvrCopy := b.genesisValidatorsRoot

	switch b.version {
	case version.Capella:
		return &zondpb.BeaconState{
			GenesisTime:                  b.genesisTime,
			GenesisValidatorsRoot:        gvrCopy[:],
			Slot:                         b.slot,
			Fork:                         b.fork,
			LatestBlockHeader:            b.latestBlockHeader,
			BlockRoots:                   b.blockRoots.Slice(),
			StateRoots:                   b.stateRoots.Slice(),
			HistoricalRoots:              b.historicalRoots.Slice(),
			Zond1Data:                    b.zond1Data,
			Zond1DataVotes:               b.zond1DataVotes,
			Zond1DepositIndex:            b.zond1DepositIndex,
			Validators:                   b.validators,
			Balances:                     b.balances,
			RandaoMixes:                  b.randaoMixes.Slice(),
			Slashings:                    b.slashings,
			PreviousEpochParticipation:   b.previousEpochParticipation,
			CurrentEpochParticipation:    b.currentEpochParticipation,
			JustificationBits:            b.justificationBits,
			PreviousJustifiedCheckpoint:  b.previousJustifiedCheckpoint,
			CurrentJustifiedCheckpoint:   b.currentJustifiedCheckpoint,
			FinalizedCheckpoint:          b.finalizedCheckpoint,
			InactivityScores:             b.inactivityScores,
			CurrentSyncCommittee:         b.currentSyncCommittee,
			NextSyncCommittee:            b.nextSyncCommittee,
			LatestExecutionPayloadHeader: b.latestExecutionPayloadHeader,
			NextWithdrawalIndex:          b.nextWithdrawalIndex,
			NextWithdrawalValidatorIndex: b.nextWithdrawalValidatorIndex,
			HistoricalSummaries:          b.historicalSummaries,
		}
	default:
		return nil
	}
}

// ToProto the beacon state into a protobuf for usage.
func (b *BeaconState) ToProto() interface{} {
	if b == nil {
		return nil
	}

	b.lock.RLock()
	defer b.lock.RUnlock()

	gvrCopy := b.genesisValidatorsRoot

	switch b.version {
	case version.Capella:
		return &zondpb.BeaconState{
			GenesisTime:                  b.genesisTime,
			GenesisValidatorsRoot:        gvrCopy[:],
			Slot:                         b.slot,
			Fork:                         b.forkVal(),
			LatestBlockHeader:            b.latestBlockHeaderVal(),
			BlockRoots:                   b.blockRoots.Slice(),
			StateRoots:                   b.stateRoots.Slice(),
			HistoricalRoots:              b.historicalRoots.Slice(),
			Zond1Data:                    b.zond1DataVal(),
			Zond1DataVotes:               b.zond1DataVotesVal(),
			Zond1DepositIndex:            b.zond1DepositIndex,
			Validators:                   b.validatorsVal(),
			Balances:                     b.balancesVal(),
			RandaoMixes:                  b.randaoMixes.Slice(),
			Slashings:                    b.slashingsVal(),
			PreviousEpochParticipation:   b.previousEpochParticipationVal(),
			CurrentEpochParticipation:    b.currentEpochParticipationVal(),
			JustificationBits:            b.justificationBitsVal(),
			PreviousJustifiedCheckpoint:  b.previousJustifiedCheckpointVal(),
			CurrentJustifiedCheckpoint:   b.currentJustifiedCheckpointVal(),
			FinalizedCheckpoint:          b.finalizedCheckpointVal(),
			InactivityScores:             b.inactivityScoresVal(),
			CurrentSyncCommittee:         b.currentSyncCommitteeVal(),
			NextSyncCommittee:            b.nextSyncCommitteeVal(),
			LatestExecutionPayloadHeader: b.latestExecutionPayloadHeaderVal(),
			NextWithdrawalIndex:          b.nextWithdrawalIndex,
			NextWithdrawalValidatorIndex: b.nextWithdrawalValidatorIndex,
			HistoricalSummaries:          b.historicalSummariesVal(),
		}
	default:
		return nil
	}
}

// StateRoots kept track of in the beacon state.
func (b *BeaconState) StateRoots() [][]byte {
	if b.stateRoots == nil {
		return nil
	}

	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.stateRoots.Slice()
}

// StateRootAtIndex retrieves a specific state root based on an
// input index value.
func (b *BeaconState) StateRootAtIndex(idx uint64) ([]byte, error) {
	if b.stateRoots == nil {
		return nil, nil
	}

	b.lock.RLock()
	defer b.lock.RUnlock()

	r, err := b.stateRootAtIndex(idx)
	if err != nil {
		return nil, err
	}
	return r[:], nil
}

// stateRootAtIndex retrieves a specific state root based on an
// input index value.
// This assumes that a lock is already held on BeaconState.
func (b *BeaconState) stateRootAtIndex(idx uint64) ([32]byte, error) {
	if uint64(len(b.stateRoots)) <= idx {
		return [32]byte{}, fmt.Errorf("index %d out of range", idx)
	}
	return b.stateRoots[idx], nil
}

// ProtobufBeaconStateCapella transforms an input into beacon state Capella in the form of protobuf.
// Error is returned if the input is not type protobuf beacon state.
func ProtobufBeaconStateCapella(s interface{}) (*zondpb.BeaconState, error) {
	pbState, ok := s.(*zondpb.BeaconState)
	if !ok {
		return nil, errors.New("input is not type pb.BeaconState")
	}
	return pbState, nil
}
