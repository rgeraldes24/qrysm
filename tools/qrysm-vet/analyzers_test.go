package main

import (
	"testing"

	"github.com/theQRL/qrysm/testing/require"
)

func TestAllAnalyzers(t *testing.T) {
	analyzers := allAnalyzers()
	require.Equal(t, len(qrysmAnalyzers)+len(goAnalyzers), len(analyzers))

	names := make(map[string]bool, len(analyzers))
	for _, a := range analyzers {
		require.Equal(t, false, names[a.Name], "duplicate analyzer %s", a.Name)
		names[a.Name] = true
	}
}
