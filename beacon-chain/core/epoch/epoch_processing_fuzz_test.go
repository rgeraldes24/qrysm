package epoch

// TODO(rgeraldes24) - benchmark epoch transition(altair) instead since this method is not used anymore
/*
func TestFuzzFinalUpdates_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	base := &zondpb.BeaconState{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(base)
		s, err := state_native.InitializeFromProtoUnsafeCapella(base)
		require.NoError(t, err)
		_, err = ProcessFinalUpdates(s)
		_ = err
	}
}
*/
