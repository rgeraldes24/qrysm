package endtoend

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/jung-kurt/gofpdf"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/runtime/version"
	ev "github.com/theQRL/qrysm/testing/endtoend/evaluators"
	e2eParams "github.com/theQRL/qrysm/testing/endtoend/params"
	"github.com/theQRL/qrysm/testing/endtoend/types"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/time/slots"
)

func TestEndToEnd_Reports_MinimalConfig(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	e2eConfig := params.E2ETestConfig().Copy()
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
	beaconNodeCount := 3
	beaconNodeCountStr, present := os.LookupEnv("E2E_BEACON_NODE_COUNT")
	if present {
		beaconNodeCount, err = strconv.Atoi(beaconNodeCountStr)
		require.NoError(t, err)
	}
	require.NoError(t, e2eParams.Init(t, beaconNodeCount))

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

	tracingPort := e2eParams.TestParams.Ports.JaegerTracingPort
	tracingEndpoint := fmt.Sprintf("127.0.0.1:%d", tracingPort)
	evals := []types.Evaluator{
		ev.PeersConnect,
		ev.HealthzCheck,
		// ev.MetricsCheck,
		ev.ValidatorsAreActive,
		ev.AllValidatorsParticipating(1),
		ev.FinalizationOccurs(3),
		ev.ColdStateCheckpoint,
		// ev.APIMiddlewareVerifyIntegrity,
		// ev.APIGatewayV1Alpha1VerifyIntegrity,
		ev.FinishedSyncing,
		ev.AllNodesHaveSameHead,
	}

	testConfig := &types.E2EConfig{
		BeaconFlags: []string{
			fmt.Sprintf("--slots-per-archive-point=%d", params.BeaconConfig().SlotsPerEpoch*16),
			fmt.Sprintf("--tracing-endpoint=http://%s", tracingEndpoint),
			"--enable-tracing",
			"--trace-sample-fraction=1.0",
		},
		ValidatorFlags:      []string{},
		EpochsToRun:         uint64(epochsToRun),
		TestSync:            false,
		TestFeature:         false,
		TestDeposits:        false,
		UseFixedPeerIDs:     true,
		UseQrysmShValidator: false,
		UsePprof:            false,
		RunMetrics:          true,
		Evaluators:          evals,
		TracingSinkEndpoint: tracingEndpoint,
		EvalInterceptor:     defaultInterceptor,
		Seed:                int64(seed),
	}

	newTestRunner(t, testConfig).run()
}

type rewardHistory struct {
	startBalances map[[field_params.DilithiumPubkeyLength]byte]uint64
	prevBalance   map[[field_params.DilithiumPubkeyLength]byte]uint64
	beaconClient  zondpb.BeaconChainClient
	History       [][]string `json:"history"`
	// TotalETH      string                                                  `json:"total_eth"`
	// TotalCurrency string     `json:"total_currency"`
	// Validators []uint64 `json:"validators"`
}

func (r *rewardHistory) LogValidatorGainsAndLosses(ctx context.Context, slot primitives.Slot, indices []primitives.ValidatorIndex) error {
	if !slots.IsEpochEnd(slot) || slot <= params.BeaconConfig().SlotsPerEpoch {
		// Do nothing unless we are at the end of the epoch, and not in the first epoch.
		return nil
	}

	req := &zondpb.ValidatorPerformanceRequest{
		Indices: indices,
	}
	resp, err := r.beaconClient.GetValidatorPerformance(ctx, req)
	if err != nil {
		return err
	}

	prevEpoch := primitives.Epoch(0)
	if slot >= params.BeaconConfig().SlotsPerEpoch {
		prevEpoch = primitives.Epoch(slot/params.BeaconConfig().SlotsPerEpoch) - 1
	}
	for i, pubKey := range resp.PublicKeys {
		r.logForEachValidator(i, pubKey, indices[i], resp, slot, prevEpoch)
	}

	return nil
}

func (r *rewardHistory) logForEachValidator(index int, pubKey []byte, valIdx primitives.ValidatorIndex, resp *zondpb.ValidatorPerformanceResponse, slot primitives.Slot, prevEpoch primitives.Epoch) {
	// truncatedKey := fmt.Sprintf("%#x", bytesutil.Trunc(pubKey))
	pubKeyBytes := bytesutil.ToBytes2592(pubKey)
	if slot < params.BeaconConfig().SlotsPerEpoch {
		r.prevBalance[pubKeyBytes] = params.BeaconConfig().MaxEffectiveBalance
	}

	// Safely load data from response with slice out of bounds checks. The server should return
	// the response with all slices of equal length, but the validator could panic if the server
	// did not do so for whatever reason.
	var balBeforeEpoch uint64
	var balAfterEpoch uint64
	if index < len(resp.BalancesBeforeEpochTransition) {
		balBeforeEpoch = resp.BalancesBeforeEpochTransition[index]
	}
	if index < len(resp.BalancesAfterEpochTransition) {
		balAfterEpoch = resp.BalancesAfterEpochTransition[index]
	}
	if _, ok := r.startBalances[pubKeyBytes]; !ok {
		r.startBalances[pubKeyBytes] = balBeforeEpoch
	}

	gweiPerEth := float64(params.BeaconConfig().GweiPerEth)
	if r.prevBalance[pubKeyBytes] > 0 {
		newBalance := float64(balAfterEpoch) / gweiPerEth
		prevBalance := float64(balBeforeEpoch) / gweiPerEth
		startBalance := float64(r.startBalances[pubKeyBytes]) / gweiPerEth
		// percentNet := (newBalance - prevBalance) / prevBalance
		// percentSinceStart := (newBalance - startBalance) / startBalance

		// previousEpochSummaryFields := logrus.Fields{
		// 	"pubKey":       truncatedKey,
		// 	"epoch":        prevEpoch,
		// 	"startBalance": startBalance,
		// 	"oldBalance":   prevBalance,
		// 	"newBalance":   newBalance,
		// 	"percentChange":           fmt.Sprintf("%.5f%%", percentNet*100),
		// 	"percentChangeSinceStart": fmt.Sprintf("%.5f%%", percentSinceStart*100),
		// }

		// if index < len(resp.InactivityScores) {
		// 	previousEpochSummaryFields["inactivityScore"] = resp.InactivityScores[index]
		// }

		r.History = append(r.History, []string{strconv.FormatUint(uint64(prevEpoch), 10), strconv.FormatUint(uint64(valIdx), 10), strconv.FormatFloat(startBalance, 'E', -1, 64), strconv.FormatFloat(prevBalance, 'E', -1, 64), strconv.FormatFloat(newBalance, 'E', -1, 64)})
		// logrus.WithField("testing", "endtoend").WithFields(previousEpochSummaryFields).Info("Previous epoch summary")
	}
	r.prevBalance[pubKeyBytes] = balBeforeEpoch
}

func GeneratePdfReport(hist rewardHistory, currency string) error {

	data := hist.History

	if !(len(data) > 0) {
		return fmt.Errorf("Can't generate PDF for Empty Slice")
	}

	// sort.Slice(data, func(p, q int) bool {
	// 	i, err := time.Parse("2006-01-02", data[p][0])
	// 	if err != nil {
	// 		return false
	// 	}

	// 	i2, err := time.Parse("2006-01-02", data[q][0])
	// 	if err != nil {
	// 		return false
	// 	}
	// 	return i2.Before(i)
	// })

	// validators := hist.Validators

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTopMargin(15)
	pdf.SetHeaderFuncMode(func() {
		pdf.SetY(5)
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(80, 0, "")
		pdf.CellFormat(30, 10, fmt.Sprintf("Validator Income History (Epochs %s - %s)", data[0][0], data[len(data)-1][0]), "", 0, "C", false, 0, "")
		// pdf.Ln(-1)
	}, true)

	pdf.AddPage()
	pdf.SetFont("Times", "", 9)

	// generating the table
	const (
		colCount = 5
		colWd    = 40.0
		marginH  = 5.0
		lineHt   = 5.5
		maxHt    = 5
	)

	pdf.SetTextColor(24, 24, 24)
	pdf.SetFillColor(255, 255, 255)
	// pdf.Ln(-1)
	// pdf.CellFormat(0, maxHt, fmt.Sprintf("Income For Timeframe %s", hist.TotalETH), "", 0, "CM", true, 0, "")

	header := [colCount]string{"Epoch", "Val Idx", "Start Balance (ETH)", "Prev Balance (ETH)", "New Balance (ETH)"}

	// pdf.SetMargins(marginH, marginH, marginH)
	pdf.Ln(10)
	pdf.SetTextColor(224, 224, 224)
	pdf.SetFillColor(64, 64, 64)
	pdf.Cell(-5, 0, "")
	for col := 0; col < colCount; col++ {
		pdf.CellFormat(colWd, maxHt, header[col], "1", 0, "CM", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(24, 24, 24)
	pdf.SetFillColor(255, 255, 255)

	// Rows
	y := pdf.GetY()

	for i, row := range data {
		pdf.SetTextColor(24, 24, 24)
		pdf.SetFillColor(255, 255, 255)
		x := marginH
		if i%47 == 0 && i != 0 {
			pdf.AddPage()
			y = pdf.GetY()
		}
		for col := 0; col < colCount; col++ {
			if i%2 != 0 {
				pdf.SetFillColor(191, 191, 191)
			}
			pdf.Rect(x, y, colWd, maxHt, "D")
			cellY := y
			pdf.SetXY(x, cellY)
			pdf.CellFormat(colWd, maxHt, row[col], "", 0,
				"LM", true, 0, "")
			// cellY += lineHt
			x += colWd
		}
		y += maxHt
	}

	// adding a footer
	pdf.AliasNbPages("")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Arial", "I", 8)
		pdf.CellFormat(0, 10, fmt.Sprintf("Page %d/{nb}", pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})

	pdf.AddPage()
	pdf.SetTextColor(24, 24, 24)
	pdf.SetFillColor(255, 255, 255)
	// pdf.Ln(10)
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, maxHt, "Validators", "", 0, "CM", true, 0, "")
	pdf.Ln(10)
	pdf.SetFont("Times", "", 9)

	const (
		vColCount = 4
		vColWd    = 50.0
	)
	vHeader := [vColCount]string{"Index", "Activation Balance", "Balance", "Last Attestation"}

	// pdf.SetMargins(marginH, marginH, marginH)
	// pdf.Ln(10)
	pdf.SetTextColor(224, 224, 224)
	pdf.SetFillColor(64, 64, 64)
	pdf.Cell(-5, 0, "")
	for col := 0; col < vColCount; col++ {
		pdf.CellFormat(vColWd, maxHt, vHeader[col], "1", 0, "CM", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(24, 24, 24)
	pdf.SetFillColor(255, 255, 255)

	// y = pdf.GetY()

	// for i, row := range getValidatorDetails(validators) {
	// 	pdf.SetTextColor(24, 24, 24)
	// 	pdf.SetFillColor(255, 255, 255)
	// 	x := marginH

	// 	if i%47 == 0 && i != 0 {
	// 		pdf.AddPage()
	// 		y = pdf.GetY()
	// 	}

	// 	for col := 0; col < vColCount; col++ {
	// 		if i%2 != 0 {
	// 			pdf.SetFillColor(191, 191, 191)
	// 		}
	// 		pdf.Rect(x, y, vColWd, maxHt, "D")
	// 		cellY := y
	// 		pdf.SetXY(x, cellY)
	// 		pdf.CellFormat(vColWd, maxHt, row[col], "", 0,
	// 			"LM", true, 0, "")
	// 		cellY += lineHt
	// 		x += vColWd
	// 	}
	// 	y += maxHt
	// }

	// adding a footer
	pdf.AliasNbPages("")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Arial", "I", 8)
		pdf.CellFormat(0, 10, fmt.Sprintf("Page %d/{nb}", pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdfPath := path.Join(e2eParams.TestParams.TestPath, fmt.Sprintf("validator_income-%d.pdf", time.Now().Unix()))
	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		return err
	}
	fmt.Printf("PDF available @ %s\n", pdfPath)

	return nil
}

func newRewardHistory(beaconClient zondpb.BeaconChainClient) *rewardHistory {
	return &rewardHistory{
		startBalances: make(map[[2592]byte]uint64),
		prevBalance:   map[[2592]byte]uint64{},
		beaconClient:  beaconClient,
		History:       make([][]string, 0),
	}
}

// SlotDeadline is the start time of the next slot.
func SlotDeadline(slot primitives.Slot, genesisTime time.Time) time.Time {
	secs := time.Duration((slot + 1).Mul(params.BeaconConfig().SecondsPerSlot))
	return genesisTime.Add(secs * time.Second)
}
