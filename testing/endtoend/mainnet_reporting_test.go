package endtoend

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/runtime/version"
	ev "github.com/theQRL/qrysm/testing/endtoend/evaluators"
	"github.com/theQRL/qrysm/testing/endtoend/types"
	"github.com/theQRL/qrysm/testing/require"
)

func TestEndToEnd_Reporting_MainnetConfig(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	e2eConfig := params.E2EMainnetTestConfig().Copy()
	e2eConfig.SecondsPerSlot = 15
	e2eConfig.MinGenesisActiveValidatorCount = 2625

	validatorCountStr, present := os.LookupEnv("E2E_GENESIS_VALIDATOR_COUNT")
	if present {
		validatorCount, err := strconv.Atoi(validatorCountStr)
		require.NoError(t, err)
		e2eConfig.MinGenesisActiveValidatorCount = uint64(validatorCount)
		e2eConfig.MaxValidatorsPerWithdrawalsSweep = uint64(validatorCount) / 2
	}
	require.NoError(t, params.SetActive(types.StartAt(version.Capella, e2eConfig)))

	var err error
	// Run for 12 epochs if not in long-running to confirm long-running has no issues.
	epochsToRun := 12
	epochStr, longRunning := os.LookupEnv("E2E_EPOCHS")
	if longRunning {
		epochsToRun, err = strconv.Atoi(epochStr)
		require.NoError(t, err)
	}
	seed := 0
	seedStr, isValid := os.LookupEnv("E2E_SEED")
	if isValid {
		seed, err = strconv.Atoi(seedStr)
		require.NoError(t, err)
	}

	evals := []types.Evaluator{
		ev.PeersConnect,
		ev.HealthzCheck,
		ev.MetricsCheck,
		ev.ValidatorsAreActive,
		// ev.AllValidatorsParticipating,
		ev.FinalizationOccurs(3),
		ev.ColdStateCheckpoint,
		ev.APIMiddlewareVerifyIntegrity,
		ev.APIGatewayV1Alpha1VerifyIntegrity,
		ev.FinishedSyncing,
		ev.AllNodesHaveSameHead,
	}
	testConfig := &types.E2EConfig{
		BeaconFlags: []string{
			fmt.Sprintf("--slots-per-archive-point=%d", params.BeaconConfig().SlotsPerEpoch*16),
		},
		ValidatorFlags:      []string{},
		EpochsToRun:         uint64(epochsToRun),
		TestSync:            false,
		TestFeature:         false,
		TestDeposits:        false,
		UseFixedPeerIDs:     true,
		UseQrysmShValidator: false,
		UsePprof:            false,
		Evaluators:          evals,
		EvalInterceptor:     defaultInterceptor,
		Seed:                int64(seed),
	}

	return newTestRunner(t, testConfig)
}
