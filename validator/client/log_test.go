package client

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/keymanager"
)

func TestLogSigningFailure(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantLevel  logrus.Level
		wantReason string
	}{
		{
			name:      "local failure",
			err:       errors.New("could not sign"),
			wantLevel: logrus.ErrorLevel,
		},
		{
			name:       "signer fault with reason",
			err:        signerRefusal{reason: "SIGNATURE_FAULT"},
			wantLevel:  logrus.ErrorLevel,
			wantReason: "SIGNATURE_FAULT",
		},
		{
			name:       "slashing protection refusal",
			err:        fmt.Errorf("sign: %w", fmt.Errorf("%w: %w", keymanager.ErrSigningRefused, signerRefusal{reason: "SLASHING_DOUBLE_VOTE"})),
			wantLevel:  logrus.WarnLevel,
			wantReason: "SLASHING_DOUBLE_VOTE",
		},
		{
			name:       "clock bound refusal",
			err:        fmt.Errorf("%w: %w", keymanager.ErrSigningRefused, signerRefusal{reason: "CLOCK_BOUND"}),
			wantLevel:  logrus.WarnLevel,
			wantReason: "CLOCK_BOUND",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, hook := logTest.NewNullLogger()
			logSigningFailure(logrus.NewEntry(logger), tt.err, "Could not sign attestation")
			entry := hook.LastEntry()
			require.NotNil(t, entry)
			require.Equal(t, "Could not sign attestation", entry.Message)
			require.Equal(t, tt.wantLevel, entry.Level)
			reason, ok := entry.Data["reason"]
			require.Equal(t, tt.wantReason != "", ok)
			if ok {
				require.Equal(t, tt.wantReason, reason)
			}
		})
	}
}
