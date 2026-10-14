package apimiddleware

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/api/gateway/apimiddleware"
	qrlpb "github.com/theQRL/qrysm/proto/qrl/v1"
)

type tempSyncCommitteesResponseJson struct {
	Data *tempSyncCommitteeValidatorsJson `json:"data"`
}

type tempSyncCommitteeValidatorsJson struct {
	Validators          []string                              `json:"validators"`
	ValidatorAggregates []*tempSyncSubcommitteeValidatorsJson `json:"validator_aggregates"`
}

type tempSyncSubcommitteeValidatorsJson struct {
	Validators []string `json:"validators"`
}

// https://ethereum.github.io/beacon-APIs/?urls.primaryName=v2.0.0#/Beacon/getEpochSyncCommittees returns validator_aggregates as a nested array.
// grpc-gateway returns a struct with nested fields which we have to transform into a plain 2D array.
func prepareValidatorAggregates(body []byte, responseContainer any) (apimiddleware.RunDefault, apimiddleware.ErrorJson) {
	tempContainer := &tempSyncCommitteesResponseJson{}
	if err := json.Unmarshal(body, tempContainer); err != nil {
		return false, apimiddleware.InternalServerErrorWithMessage(err, "could not unmarshal response into temp container")
	}
	container, ok := responseContainer.(*SyncCommitteesResponseJson)
	if !ok {
		return false, apimiddleware.InternalServerError(errors.New("container is not of the correct type"))
	}

	container.Data = &SyncCommitteeValidatorsJson{}
	container.Data.Validators = tempContainer.Data.Validators
	container.Data.ValidatorAggregates = make([][]string, len(tempContainer.Data.ValidatorAggregates))
	for i, srcValAgg := range tempContainer.Data.ValidatorAggregates {
		dstValAgg := make([]string, len(srcValAgg.Validators))
		copy(dstValAgg, tempContainer.Data.ValidatorAggregates[i].Validators)
		container.Data.ValidatorAggregates[i] = dstValAgg
	}

	return false, nil
}

type zondBlockResponseJson struct {
	Version             string                     `json:"version"`
	Data                *SignedBeaconBlockZondJson `json:"data"`
	ExecutionOptimistic bool                       `json:"execution_optimistic"`
	Finalized           bool                       `json:"finalized"`
}

type zondBlindedBlockResponseJson struct {
	Version             string                            `json:"version" enum:"true"`
	Data                *SignedBlindedBeaconBlockZondJson `json:"data"`
	ExecutionOptimistic bool                              `json:"execution_optimistic"`
	Finalized           bool                              `json:"finalized"`
}

func serializeBlock(response any) (apimiddleware.RunDefault, []byte, apimiddleware.ErrorJson) {
	respContainer, ok := response.(*BlockResponseJson)
	if !ok {
		return false, nil, apimiddleware.InternalServerError(errors.New("container is not of the correct type"))
	}

	var actualRespContainer any
	switch {
	case strings.EqualFold(respContainer.Version, strings.ToLower(qrlpb.Version_ZOND.String())):
		actualRespContainer = &zondBlockResponseJson{
			Version: respContainer.Version,
			Data: &SignedBeaconBlockZondJson{
				Message:   respContainer.Data.ZondBlock,
				Signature: respContainer.Data.Signature,
			},
			ExecutionOptimistic: respContainer.ExecutionOptimistic,
			Finalized:           respContainer.Finalized,
		}
	default:
		return false, nil, apimiddleware.InternalServerError(fmt.Errorf("unsupported block version '%s'", respContainer.Version))
	}

	j, err := json.Marshal(actualRespContainer)
	if err != nil {
		return false, nil, apimiddleware.InternalServerErrorWithMessage(err, "could not marshal response")
	}
	return false, j, nil
}

func serializeBlindedBlock(response any) (apimiddleware.RunDefault, []byte, apimiddleware.ErrorJson) {
	respContainer, ok := response.(*BlindedBlockResponseJson)
	if !ok {
		return false, nil, apimiddleware.InternalServerError(errors.New("container is not of the correct type"))
	}

	var actualRespContainer any
	switch {
	case strings.EqualFold(respContainer.Version, strings.ToLower(qrlpb.Version_ZOND.String())):
		actualRespContainer = &zondBlindedBlockResponseJson{
			Version: respContainer.Version,
			Data: &SignedBlindedBeaconBlockZondJson{
				Message:   respContainer.Data.ZondBlock,
				Signature: respContainer.Data.Signature,
			},
			ExecutionOptimistic: respContainer.ExecutionOptimistic,
			Finalized:           respContainer.Finalized,
		}
	default:
		return false, nil, apimiddleware.InternalServerError(fmt.Errorf("unsupported block version '%s'", respContainer.Version))
	}

	j, err := json.Marshal(actualRespContainer)
	if err != nil {
		return false, nil, apimiddleware.InternalServerErrorWithMessage(err, "could not marshal response")
	}
	return false, j, nil
}

type zondStateResponseJson struct {
	Version string               `json:"version" enum:"true"`
	Data    *BeaconStateZondJson `json:"data"`
}

func serializeState(response any) (apimiddleware.RunDefault, []byte, apimiddleware.ErrorJson) {
	respContainer, ok := response.(*BeaconStateResponseJson)
	if !ok {
		return false, nil, apimiddleware.InternalServerError(errors.New("container is not of the correct type"))
	}

	var actualRespContainer any
	switch {
	case strings.EqualFold(respContainer.Version, strings.ToLower(qrlpb.Version_ZOND.String())):
		actualRespContainer = &zondStateResponseJson{
			Version: respContainer.Version,
			Data:    respContainer.Data.ZondState,
		}
	default:
		return false, nil, apimiddleware.InternalServerError(fmt.Errorf("unsupported state version '%s'", respContainer.Version))
	}

	j, err := json.Marshal(actualRespContainer)
	if err != nil {
		return false, nil, apimiddleware.InternalServerErrorWithMessage(err, "could not marshal response")
	}
	return false, j, nil
}

type zondProduceBlockResponseJson struct {
	Version string               `json:"version" enum:"true"`
	Data    *BeaconBlockZondJson `json:"data"`
}

type zondProduceBlindedBlockResponseJson struct {
	Version string                      `json:"version" enum:"true"`
	Data    *BlindedBeaconBlockZondJson `json:"data"`
}

func serializeProducedBlock(response any) (apimiddleware.RunDefault, []byte, apimiddleware.ErrorJson) {
	respContainer, ok := response.(*ProduceBlockResponseJson)
	if !ok {
		return false, nil, apimiddleware.InternalServerError(errors.New("container is not of the correct type"))
	}

	var actualRespContainer any
	switch {
	case strings.EqualFold(respContainer.Version, strings.ToLower(qrlpb.Version_ZOND.String())):
		actualRespContainer = &zondProduceBlockResponseJson{
			Version: respContainer.Version,
			Data:    respContainer.Data.ZondBlock,
		}
	default:
		return false, nil, apimiddleware.InternalServerError(fmt.Errorf("unsupported block version '%s'", respContainer.Version))
	}

	j, err := json.Marshal(actualRespContainer)
	if err != nil {
		return false, nil, apimiddleware.InternalServerErrorWithMessage(err, "could not marshal response")
	}
	return false, j, nil
}

func serializeProducedBlindedBlock(response any) (apimiddleware.RunDefault, []byte, apimiddleware.ErrorJson) {
	respContainer, ok := response.(*ProduceBlindedBlockResponseJson)
	if !ok {
		return false, nil, apimiddleware.InternalServerError(errors.New("container is not of the correct type"))
	}

	var actualRespContainer any
	switch {
	case strings.EqualFold(respContainer.Version, strings.ToLower(qrlpb.Version_ZOND.String())):
		actualRespContainer = &zondProduceBlindedBlockResponseJson{
			Version: respContainer.Version,
			Data:    respContainer.Data.ZondBlock,
		}
	default:
		return false, nil, apimiddleware.InternalServerError(fmt.Errorf("unsupported block version '%s'", respContainer.Version))
	}

	j, err := json.Marshal(actualRespContainer)
	if err != nil {
		return false, nil, apimiddleware.InternalServerErrorWithMessage(err, "could not marshal response")
	}
	return false, j, nil
}

func prepareForkChoiceResponse(response any) (apimiddleware.RunDefault, []byte, apimiddleware.ErrorJson) {
	dump, ok := response.(*ForkChoiceDumpJson)
	if !ok {
		return false, nil, apimiddleware.InternalServerError(errors.New("response is not of the correct type"))
	}

	nodes := make([]*ForkChoiceNodeResponseJson, len(dump.ForkChoiceNodes))
	for i, n := range dump.ForkChoiceNodes {
		nodes[i] = &ForkChoiceNodeResponseJson{
			Slot:               n.Slot,
			BlockRoot:          n.BlockRoot,
			ParentRoot:         n.ParentRoot,
			JustifiedEpoch:     n.JustifiedEpoch,
			FinalizedEpoch:     n.FinalizedEpoch,
			Weight:             n.Weight,
			Validity:           n.Validity,
			ExecutionBlockHash: n.ExecutionBlockHash,
			ExtraData: &ForkChoiceNodeExtraDataJson{
				UnrealizedJustifiedEpoch: n.UnrealizedJustifiedEpoch,
				UnrealizedFinalizedEpoch: n.UnrealizedFinalizedEpoch,
				Balance:                  n.Balance,
				ExecutionOptimistic:      n.ExecutionOptimistic,
				TimeStamp:                n.TimeStamp,
			},
		}
	}
	forkChoice := &ForkChoiceResponseJson{
		JustifiedCheckpoint: dump.JustifiedCheckpoint,
		FinalizedCheckpoint: dump.FinalizedCheckpoint,
		ForkChoiceNodes:     nodes,
		ExtraData: &ForkChoiceResponseExtraDataJson{
			BestJustifiedCheckpoint:       dump.BestJustifiedCheckpoint,
			UnrealizedJustifiedCheckpoint: dump.UnrealizedJustifiedCheckpoint,
			UnrealizedFinalizedCheckpoint: dump.UnrealizedFinalizedCheckpoint,
			ProposerBoostRoot:             dump.ProposerBoostRoot,
			PreviousProposerBoostRoot:     dump.PreviousProposerBoostRoot,
			HeadRoot:                      dump.HeadRoot,
		},
	}

	result, err := json.Marshal(forkChoice)
	if err != nil {
		return false, nil, apimiddleware.InternalServerError(errors.New("could not marshal fork choice to JSON"))
	}
	return false, result, nil
}
