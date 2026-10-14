package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	validatormock "github.com/theQRL/qrysm/testing/validator-mock"
)

func TestValidatorClientDutyDependentRootsProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := validatormock.NewMockValidatorClient(ctrl)
	provider := &validatorClientDutyDependentRootsProvider{validatorClient: client, timeout: time.Second}

	client.EXPECT().GetDuties(
		gomock.Any(),
		&qrysmpb.DutiesRequest{Epoch: 5},
	).Return(&qrysmpb.DutiesResponse{
		PreviousDutyDependentRoot: []byte("previous-root"),
		CurrentDutyDependentRoot:  []byte("current-root"),
	}, nil)
	previousRoot, currentRoot, err := provider.GetDutyDependentRoots(context.Background(), primitives.Epoch(5))
	require.NoError(t, err)
	assert.DeepEqual(t, []byte("previous-root"), previousRoot)
	assert.DeepEqual(t, []byte("current-root"), currentRoot)

	client.EXPECT().GetDuties(gomock.Any(), gomock.Any()).Return(nil, errors.New("unavailable"))
	_, _, err = provider.GetDutyDependentRoots(context.Background(), primitives.Epoch(5))
	require.ErrorContains(t, "unavailable", err)
}
