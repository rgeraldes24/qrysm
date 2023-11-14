package grpc_api

/*
func TestWaitForChainStart_StreamSetupFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	beaconNodeValidatorClient := mock2.NewMockBeaconNodeValidatorClient(ctrl)
	beaconNodeValidatorClient.EXPECT().WaitForChainStart(
		gomock.Any(),
		gomock.Any(),
	).Return(nil, errors.New("failed stream"))

	validatorClient := &grpcValidatorClient{beaconNodeValidatorClient}
	_, err := validatorClient.WaitForChainStart(context.Background(), &emptypb.Empty{})
	want := "could not setup beacon chain ChainStart streaming client"
	assert.ErrorContains(t, want, err)
}
*/
