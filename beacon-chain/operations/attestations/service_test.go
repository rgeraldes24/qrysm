package attestations

import (
	"context"
	"testing"

	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

func TestStop_OK(t *testing.T) {
	s, err := NewService(context.Background(), &Config{})
	require.NoError(t, err)
	require.NoError(t, s.Stop(), "Unable to stop attestation pool service")
	assert.ErrorContains(t, context.Canceled.Error(), s.ctx.Err(), "Context was not canceled")
}
