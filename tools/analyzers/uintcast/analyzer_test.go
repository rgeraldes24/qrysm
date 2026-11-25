package uintcast_test

import (
	"testing"

	"github.com/theQRL/qrysm/tools/analyzers/uintcast"
	"golang.org/x/tools/go/analysis/analysistest"
)


func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), uintcast.Analyzer)
}
