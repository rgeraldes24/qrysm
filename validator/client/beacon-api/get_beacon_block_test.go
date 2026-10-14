package beacon_api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/beacon-chain/rpc/apimiddleware"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/client/beacon-api/mock"
	test_helpers "github.com/theQRL/qrysm/validator/client/beacon-api/test-helpers"
)

func TestGetBeaconBlock_RequestFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	jsonRestHandler := mock.NewMockjsonRestHandler(ctrl)
	jsonRestHandler.EXPECT().GetRestJsonResponse(
		ctx,
		gomock.Any(),
		gomock.Any(),
	).Return(
		nil,
		errors.New("foo error"),
	).Times(1)

	validatorClient := &beaconApiValidatorClient{jsonRestHandler: jsonRestHandler}
	_, err := validatorClient.getBeaconBlock(ctx, 1, []byte{1}, []byte{2})
	assert.ErrorContains(t, "failed to query GET REST endpoint", err)
	assert.ErrorContains(t, "foo error", err)
}

func TestGetBeaconBlock_GraffitiIs32Bytes(t *testing.T) {
	long := make([]byte, 40)
	for i := range long {
		long[i] = byte(i + 1)
	}
	tests := []struct {
		name     string
		graffiti []byte
		want     string
	}{
		{name: "short is padded", graffiti: []byte("hello"), want: "graffiti=" + hexutil.Encode(bytesutil.PadTo([]byte("hello"), 32)) + "&"},
		{name: "exact is unchanged", graffiti: long[:32], want: "graffiti=" + hexutil.Encode(long[:32]) + "&"},
		{name: "long is truncated", graffiti: long, want: "graffiti=" + hexutil.Encode(long[:32]) + "&"},
		{name: "empty is omitted", graffiti: nil, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			jsonRestHandler := mock.NewMockjsonRestHandler(ctrl)
			jsonRestHandler.EXPECT().GetRestJsonResponse(
				ctx,
				"/qrl/v1/validator/blocks/1?"+tt.want+"randao_reveal=0x01",
				gomock.Any(),
			).Return(
				nil,
				errors.New("stop"),
			).Times(1)

			validatorClient := &beaconApiValidatorClient{jsonRestHandler: jsonRestHandler}
			_, err := validatorClient.getBeaconBlock(ctx, 1, []byte{1}, tt.graffiti)
			require.ErrorContains(t, "stop", err)
		})
	}
}

func TestGetBeaconBlock_Error(t *testing.T) {
	zondBeaconBlockBytes, err := json.Marshal(apimiddleware.BeaconBlockZondJson{})
	require.NoError(t, err)

	testCases := []struct {
		name                 string
		beaconBlock          any
		expectedErrorMessage string
		consensusVersion     string
		data                 json.RawMessage
	}{
		{
			name:                 "zond block decoding failed",
			expectedErrorMessage: "failed to decode zond block response json",
			beaconBlock:          "foo",
			consensusVersion:     "zond",
			data:                 []byte{},
		},
		{
			name:                 "zond block conversion failed",
			expectedErrorMessage: "failed to get zond block",
			consensusVersion:     "zond",
			data:                 zondBeaconBlockBytes,
		},
		{
			name:                 "unsupported consensus version",
			expectedErrorMessage: "unsupported consensus version `foo`",
			consensusVersion:     "foo",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()

			jsonRestHandler := mock.NewMockjsonRestHandler(ctrl)
			jsonRestHandler.EXPECT().GetRestJsonResponse(
				ctx,
				gomock.Any(),
				&abstractProduceBlockResponseJson{},
			).SetArg(
				2,
				abstractProduceBlockResponseJson{
					Version: testCase.consensusVersion,
					Data:    testCase.data,
				},
			).Return(
				nil,
				nil,
			).Times(1)

			beaconBlockConverter := mock.NewMockbeaconBlockConverter(ctrl)
			beaconBlockConverter.EXPECT().ConvertRESTZondBlockToProto(
				gomock.Any(),
			).Return(
				nil,
				errors.New(testCase.expectedErrorMessage),
			).AnyTimes()

			validatorClient := &beaconApiValidatorClient{jsonRestHandler: jsonRestHandler, beaconBlockConverter: beaconBlockConverter}
			_, err := validatorClient.getBeaconBlock(ctx, 1, []byte{1}, []byte{2})
			assert.ErrorContains(t, testCase.expectedErrorMessage, err)
		})
	}
}

func TestGetBeaconBlock_ZondValid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	zondProtoBeaconBlock := test_helpers.GenerateProtoZondBeaconBlock()
	zondBeaconBlock := test_helpers.GenerateJsonZondBeaconBlock()
	zondBeaconBlockBytes, err := json.Marshal(zondBeaconBlock)
	require.NoError(t, err)

	const slot = primitives.Slot(1)
	randaoReveal := []byte{2}
	graffiti := bytesutil.PadTo([]byte{3}, 32)

	ctx := context.Background()

	jsonRestHandler := mock.NewMockjsonRestHandler(ctrl)
	jsonRestHandler.EXPECT().GetRestJsonResponse(
		ctx,
		fmt.Sprintf("/qrl/v1/validator/blocks/%d?graffiti=%s&randao_reveal=%s", slot, hexutil.Encode(graffiti), hexutil.Encode(randaoReveal)),
		&abstractProduceBlockResponseJson{},
	).SetArg(
		2,
		abstractProduceBlockResponseJson{
			Version: "zond",
			Data:    zondBeaconBlockBytes,
		},
	).Return(
		nil,
		nil,
	).Times(1)

	beaconBlockConverter := mock.NewMockbeaconBlockConverter(ctrl)
	beaconBlockConverter.EXPECT().ConvertRESTZondBlockToProto(
		zondBeaconBlock,
	).Return(
		zondProtoBeaconBlock,
		nil,
	).Times(1)

	validatorClient := &beaconApiValidatorClient{jsonRestHandler: jsonRestHandler, beaconBlockConverter: beaconBlockConverter}
	beaconBlock, err := validatorClient.getBeaconBlock(ctx, slot, randaoReveal, graffiti)
	require.NoError(t, err)

	expectedBeaconBlock := &qrysmpb.GenericBeaconBlock{
		Block: &qrysmpb.GenericBeaconBlock_Zond{
			Zond: zondProtoBeaconBlock,
		},
	}

	assert.DeepEqual(t, expectedBeaconBlock, beaconBlock)
}
