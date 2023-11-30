package beaconapi_evaluators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/theQRL/go-zond/common/hexutil"
	"github.com/theQRL/qrysm/v4/beacon-chain/rpc/apimiddleware"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/proto/zond/service"
	v1 "github.com/theQRL/qrysm/v4/proto/zond/v1"
	"github.com/theQRL/qrysm/v4/testing/endtoend/helpers"
	"github.com/theQRL/qrysm/v4/time/slots"
	"google.golang.org/grpc"
)

type metadata struct {
	basepath         string
	params           func(encoding string, currentEpoch primitives.Epoch) []string
	requestObject    interface{}
	qrysmResps       map[string]interface{}
	lighthouseResps  map[string]interface{}
	customEvaluation func(interface{}, interface{}) error
}

var beaconPathsAndObjects = map[string]metadata{
	"/beacon/genesis": {
		basepath: v1MiddlewarePathTemplate,
		params: func(_ string, _ primitives.Epoch) []string {
			return []string{}
		},
		qrysmResps: map[string]interface{}{
			"json": &apimiddleware.GenesisResponseJson{},
		},
		lighthouseResps: map[string]interface{}{
			"json": &apimiddleware.GenesisResponseJson{},
		},
	},
	"/beacon/states/{param1}/root": {
		basepath: v1MiddlewarePathTemplate,
		params: func(_ string, _ primitives.Epoch) []string {
			return []string{"head"}
		},
		qrysmResps: map[string]interface{}{
			"json": &apimiddleware.StateRootResponseJson{},
		},
		lighthouseResps: map[string]interface{}{
			"json": &apimiddleware.StateRootResponseJson{},
		},
	},
	"/beacon/states/{param1}/finality_checkpoints": {
		basepath: v1MiddlewarePathTemplate,
		params: func(_ string, _ primitives.Epoch) []string {
			return []string{"head"}
		},
		qrysmResps: map[string]interface{}{
			"json": &apimiddleware.StateFinalityCheckpointResponseJson{},
		},
		lighthouseResps: map[string]interface{}{
			"json": &apimiddleware.StateFinalityCheckpointResponseJson{},
		},
	},
	"/beacon/blocks/{param1}": {
		basepath: v1MiddlewarePathTemplate,
		params: func(t string, e primitives.Epoch) []string {
			if t == "ssz" {
				if e < 4 {
					return []string{"genesis"}
				}
				return []string{"finalized"}
			}
			return []string{"head"}

		},
		qrysmResps: map[string]interface{}{
			"json": &apimiddleware.BlockResponseJson{},
			"ssz":  []byte{},
		},
		lighthouseResps: map[string]interface{}{
			"json": &apimiddleware.BlockResponseJson{},
			"ssz":  []byte{},
		},
	},
	"/beacon/states/{param1}/fork": {
		basepath: v1MiddlewarePathTemplate,
		params: func(_ string, _ primitives.Epoch) []string {
			return []string{"finalized"}
		},
		qrysmResps: map[string]interface{}{
			"json": &apimiddleware.StateForkResponseJson{},
		},
		lighthouseResps: map[string]interface{}{
			"json": &apimiddleware.StateForkResponseJson{},
		},
	},
	"/debug/beacon/states/{param1}": {
		basepath: v1MiddlewarePathTemplate,
		params: func(_ string, e primitives.Epoch) []string {
			return []string{"head"}
		},
		qrysmResps: map[string]interface{}{
			"json": &apimiddleware.BeaconStateResponseJson{},
		},
		lighthouseResps: map[string]interface{}{
			"json": &apimiddleware.BeaconStateResponseJson{},
		},
	},
	"/validator/duties/proposer/{param1}": {
		basepath: v1MiddlewarePathTemplate,
		params: func(_ string, e primitives.Epoch) []string {
			return []string{fmt.Sprintf("%v", e)}
		},
		qrysmResps: map[string]interface{}{
			"json": &apimiddleware.ProposerDutiesResponseJson{},
		},
		lighthouseResps: map[string]interface{}{
			"json": &apimiddleware.ProposerDutiesResponseJson{},
		},
		customEvaluation: func(qrysmResp interface{}, lhouseResp interface{}) error {
			castedl, ok := lhouseResp.(*apimiddleware.ProposerDutiesResponseJson)
			if !ok {
				return errors.New("failed to cast type")
			}
			if castedl.Data[0].Slot == "0" {
				// remove the first item from lighthouse data since lighthouse is returning a value despite no proposer
				// there is no proposer on slot 0 so qrysm don't return anything for slot 0
				castedl.Data = castedl.Data[1:]
			}
			return compareJSONResponseObjects(qrysmResp, castedl)
		},
	},
	"/validator/duties/attester/{param1}": {
		basepath: v1MiddlewarePathTemplate,
		params: func(_ string, e primitives.Epoch) []string {
			//ask for a future epoch to test this case
			return []string{fmt.Sprintf("%v", e+1)}
		},
		requestObject: func() []string {
			validatorIndices := make([]string, 64)
			for key := range validatorIndices {
				validatorIndices[key] = fmt.Sprintf("%d", key)
			}
			return validatorIndices
		}(),
		qrysmResps: map[string]interface{}{
			"json": &apimiddleware.AttesterDutiesResponseJson{},
		},
		lighthouseResps: map[string]interface{}{
			"json": &apimiddleware.AttesterDutiesResponseJson{},
		},
		customEvaluation: func(qrysmResp interface{}, lhouseResp interface{}) error {
			castedp, ok := lhouseResp.(*apimiddleware.AttesterDutiesResponseJson)
			if !ok {
				return errors.New("failed to cast type")
			}
			castedl, ok := lhouseResp.(*apimiddleware.AttesterDutiesResponseJson)
			if !ok {
				return errors.New("failed to cast type")
			}
			if len(castedp.Data) == 0 ||
				len(castedl.Data) == 0 ||
				len(castedp.Data) != len(castedl.Data) {
				return fmt.Errorf("attester data does not match, qrysm: %d lighthouse: %d", len(castedp.Data), len(castedl.Data))
			}
			return compareJSONResponseObjects(qrysmResp, castedl)
		},
	},
	"/beacon/headers/{param1}": {
		basepath: v1MiddlewarePathTemplate,
		params: func(_ string, e primitives.Epoch) []string {
			slot := uint64(0)
			if e > 0 {
				slot = (uint64(e) * uint64(params.BeaconConfig().SlotsPerEpoch)) - 1
			}
			return []string{fmt.Sprintf("%v", slot)}
		},
		qrysmResps: map[string]interface{}{
			"json": &apimiddleware.BlockHeaderResponseJson{},
		},
		lighthouseResps: map[string]interface{}{
			"json": &apimiddleware.BlockHeaderResponseJson{},
		},
	},
	"/node/identity": {
		basepath: v1MiddlewarePathTemplate,
		params: func(_ string, _ primitives.Epoch) []string {
			return []string{}
		},
		qrysmResps: map[string]interface{}{
			"json": &apimiddleware.IdentityResponseJson{},
		},
		lighthouseResps: map[string]interface{}{
			"json": &apimiddleware.IdentityResponseJson{},
		},
		customEvaluation: func(qrysmResp interface{}, lhouseResp interface{}) error {
			castedp, ok := qrysmResp.(*apimiddleware.IdentityResponseJson)
			if !ok {
				return errors.New("failed to cast type")
			}
			castedl, ok := lhouseResp.(*apimiddleware.IdentityResponseJson)
			if !ok {
				return errors.New("failed to cast type")
			}
			if castedp.Data == nil {
				return errors.New("qrysm node identity was empty")
			}
			if castedl.Data == nil {
				return errors.New("lighthouse node identity was empty")
			}
			return nil
		},
	},
	"/node/peers": {
		basepath: v1MiddlewarePathTemplate,
		params: func(_ string, _ primitives.Epoch) []string {
			return []string{}
		},
		qrysmResps: map[string]interface{}{
			"json": &apimiddleware.PeersResponseJson{},
		},
		lighthouseResps: map[string]interface{}{
			"json": &apimiddleware.PeersResponseJson{},
		},
		customEvaluation: func(qrysmResp interface{}, lhouseResp interface{}) error {
			castedp, ok := qrysmResp.(*apimiddleware.PeersResponseJson)
			if !ok {
				return errors.New("failed to cast type")
			}
			castedl, ok := lhouseResp.(*apimiddleware.PeersResponseJson)
			if !ok {
				return errors.New("failed to cast type")
			}
			if castedp.Data == nil {
				return errors.New("qrysm node identity was empty")
			}
			if castedl.Data == nil {
				return errors.New("lighthouse node identity was empty")
			}
			return nil
		},
	},
}

func withCompareBeaconAPIs(beaconNodeIdx int, conn *grpc.ClientConn) error {
	ctx := context.Background()
	beaconClient := service.NewBeaconChainClient(conn)
	genesisData, err := beaconClient.GetGenesis(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "error getting genesis data")
	}
	currentEpoch := slots.EpochsSinceGenesis(genesisData.Data.GenesisTime.AsTime())

	for path, meta := range beaconPathsAndObjects {
		for key := range meta.qrysmResps {
			switch key {
			case "json":
				jsonparams := meta.params("json", currentEpoch)
				apipath := pathFromParams(path, jsonparams)
				fmt.Printf("executing json api path: %s\n", apipath)
				if err := compareJSONMulticlient(beaconNodeIdx,
					meta.basepath,
					apipath,
					meta.requestObject,
					beaconPathsAndObjects[path].qrysmResps[key],
					beaconPathsAndObjects[path].lighthouseResps[key],
					meta.customEvaluation,
				); err != nil {
					return err
				}
			case "ssz":
				sszparams := meta.params("ssz", currentEpoch)
				if len(sszparams) == 0 {
					continue
				}
				apipath := pathFromParams(path, sszparams)
				fmt.Printf("executing ssz api path: %s\n", apipath)
				qrysmr, lighthouser, err := compareSSZMulticlient(beaconNodeIdx, meta.basepath, apipath)
				if err != nil {
					return err
				}
				beaconPathsAndObjects[path].qrysmResps[key] = qrysmr
				beaconPathsAndObjects[path].lighthouseResps[key] = lighthouser
			default:
				return fmt.Errorf("unknown encoding type %s", key)
			}
		}
	}
	return orderedEvaluationOnResponses(beaconPathsAndObjects, genesisData)
}

func orderedEvaluationOnResponses(beaconPathsAndObjects map[string]metadata, genesisData *v1.GenesisResponse) error {
	forkPathData := beaconPathsAndObjects["/beacon/states/{param1}/fork"]
	qrysmForkData, ok := forkPathData.qrysmResps["json"].(*apimiddleware.StateForkResponseJson)
	if !ok {
		return errors.New("failed to cast type")
	}
	lighthouseForkData, ok := forkPathData.lighthouseResps["json"].(*apimiddleware.StateForkResponseJson)
	if !ok {
		return errors.New("failed to cast type")
	}
	if qrysmForkData.Data.Epoch != lighthouseForkData.Data.Epoch {
		return fmt.Errorf("qrysm epoch %v does not match lighthouse epoch %v",
			qrysmForkData.Data.Epoch,
			lighthouseForkData.Data.Epoch)
	}

	finalizedEpoch, err := strconv.ParseUint(qrysmForkData.Data.Epoch, 10, 64)
	if err != nil {
		return err
	}
	blockPathData := beaconPathsAndObjects["/beacon/blocks/{param1}"]
	sszrspL, ok := blockPathData.qrysmResps["ssz"].([]byte)
	if !ok {
		return errors.New("failed to cast type")
	}
	sszrspP, ok := blockPathData.lighthouseResps["ssz"].([]byte)
	if !ok {
		return errors.New("failed to cast type")
	}
	if finalizedEpoch < helpers.AltairE2EForkEpoch+2 {
		blockP := &zondpb.SignedBeaconBlock{}
		blockL := &zondpb.SignedBeaconBlock{}
		if err := blockL.UnmarshalSSZ(sszrspL); err != nil {
			return errors.Wrap(err, "failed to unmarshal lighthouse ssz")
		}
		if err := blockP.UnmarshalSSZ(sszrspP); err != nil {
			return errors.Wrap(err, "failed to unmarshal rysm ssz")
		}
		if len(blockP.Signature) == 0 || len(blockL.Signature) == 0 || hexutil.Encode(blockP.Signature) != hexutil.Encode(blockL.Signature) {
			return errors.New("qrysm signature does not match lighthouse signature")
		}
	} else if finalizedEpoch >= helpers.AltairE2EForkEpoch+2 && finalizedEpoch < helpers.BellatrixE2EForkEpoch {
		blockP := &zondpb.SignedBeaconBlockAltair{}
		blockL := &zondpb.SignedBeaconBlockAltair{}
		if err := blockL.UnmarshalSSZ(sszrspL); err != nil {
			return errors.Wrap(err, "lighthouse ssz error")
		}
		if err := blockP.UnmarshalSSZ(sszrspP); err != nil {
			return errors.Wrap(err, "qrysm ssz error")
		}

		if len(blockP.Signature) == 0 || len(blockL.Signature) == 0 || hexutil.Encode(blockP.Signature) != hexutil.Encode(blockL.Signature) {
			return fmt.Errorf("qrysm response %v does not match lighthouse response %v",
				blockP,
				blockL)
		}
	} else {
		blockP := &zondpb.SignedBeaconBlockBellatrix{}
		blockL := &zondpb.SignedBeaconBlockBellatrix{}
		if err := blockL.UnmarshalSSZ(sszrspL); err != nil {
			return errors.Wrap(err, "lighthouse ssz error")
		}
		if err := blockP.UnmarshalSSZ(sszrspP); err != nil {
			return errors.Wrap(err, "qrysm ssz error")
		}

		if len(blockP.Signature) == 0 || len(blockL.Signature) == 0 || hexutil.Encode(blockP.Signature) != hexutil.Encode(blockL.Signature) {
			return fmt.Errorf("qrysm response %v does not match lighthouse response %v",
				blockP,
				blockL)
		}
	}
	blockheaderData := beaconPathsAndObjects["/beacon/headers/{param1}"]
	qrysmHeader, ok := blockheaderData.qrysmResps["json"].(*apimiddleware.BlockHeaderResponseJson)
	if !ok {
		return errors.New("failed to cast type")
	}
	proposerdutiesData := beaconPathsAndObjects["/validator/duties/proposer/{param1}"]
	qrysmDuties, ok := proposerdutiesData.qrysmResps["json"].(*apimiddleware.ProposerDutiesResponseJson)
	if !ok {
		return errors.New("failed to cast type")
	}
	if qrysmHeader.Data.Root != qrysmDuties.DependentRoot {
		fmt.Printf("current slot: %v\n", slots.CurrentSlot(uint64(genesisData.Data.GenesisTime.AsTime().Unix())))
		return fmt.Errorf("header root %s does not match duties root %s ", qrysmHeader.Data.Root, qrysmDuties.DependentRoot)
	}

	return nil
}

func compareJSONMulticlient(beaconNodeIdx int, base string, path string, requestObj, respJSONQrysm interface{}, respJSONLighthouse interface{}, customEvaluator func(interface{}, interface{}) error) error {
	if requestObj != nil {
		if err := doMiddlewareJSONPostRequest(
			base,
			path,
			beaconNodeIdx,
			requestObj,
			respJSONQrysm,
		); err != nil {
			return errors.Wrap(err, "could not perform POST request for Qrysm JSON")
		}

		if err := doMiddlewareJSONPostRequest(
			base,
			path,
			beaconNodeIdx,
			requestObj,
			respJSONLighthouse,
			"lighthouse",
		); err != nil {
			return errors.Wrap(err, "could not perform POST request for Lighthouse JSON")
		}
	} else {
		if err := doMiddlewareJSONGetRequest(
			base,
			path,
			beaconNodeIdx,
			respJSONQrysm,
		); err != nil {
			return errors.Wrap(err, "could not perform GET request for Qrysm JSON")
		}

		if err := doMiddlewareJSONGetRequest(
			base,
			path,
			beaconNodeIdx,
			respJSONLighthouse,
			"lighthouse",
		); err != nil {
			return errors.Wrap(err, "could not perform GET request for Lighthouse JSON")
		}
	}
	if customEvaluator != nil {
		return customEvaluator(respJSONQrysm, respJSONLighthouse)
	} else {
		return compareJSONResponseObjects(respJSONQrysm, respJSONLighthouse)
	}
}

func compareSSZMulticlient(beaconNodeIdx int, base string, path string) ([]byte, []byte, error) {
	sszrspL, err := doMiddlewareSSZGetRequest(
		base,
		path,
		beaconNodeIdx,
		"lighthouse",
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not perform GET request for Lighthouse SSZ")
	}

	sszrspP, err := doMiddlewareSSZGetRequest(
		base,
		path,
		beaconNodeIdx,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not perform GET request for Qrysm SSZ")
	}
	if !bytes.Equal(sszrspL, sszrspP) {
		return nil, nil, errors.New("qrysm ssz response does not match lighthouse ssz response")
	}
	return sszrspP, sszrspL, nil
}

func compareJSONResponseObjects(qrysmResp interface{}, lighthouseResp interface{}) error {
	if !reflect.DeepEqual(qrysmResp, lighthouseResp) {
		p, err := json.Marshal(qrysmResp)
		if err != nil {
			return errors.Wrap(err, "failed to marshal Qrysm response to JSON")
		}
		l, err := json.Marshal(lighthouseResp)
		if err != nil {
			return errors.Wrap(err, "failed to marshal Lighthouse response to JSON")
		}
		return fmt.Errorf("qrysm response %s does not match lighthouse response %s",
			string(p),
			string(l))
	}
	return nil
}

func pathFromParams(path string, params []string) string {
	apiPath := path
	for index := range params {
		apiPath = strings.Replace(path, fmt.Sprintf("{param%d}", index+1), params[index], 1)
	}
	return apiPath
}
