package sync

import (
	"context"
	"testing"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
)

func TestValidateWithBatchVerifier(t *testing.T) {
	_, keys, err := util.DeterministicDepositsAndKeys(10)
	assert.NoError(t, err)
	sig, err := keys[0].Sign(make([]byte, 32))
	require.NoError(t, err)
	badSig, err := keys[1].Sign(make([]byte, 32))
	require.NoError(t, err)
	validSet := &ml_dsa_87.SignatureBatch{
		Messages:     [][32]byte{{}},
		PublicKeys:   [][]ml_dsa_87.PublicKey{{keys[0].PublicKey()}},
		Signatures:   [][][]byte{{sig.Marshal()}},
		Descriptions: []string{signing.UnknownSignature},
	}
	invalidSet := &ml_dsa_87.SignatureBatch{
		Messages:     [][32]byte{{}},
		PublicKeys:   [][]ml_dsa_87.PublicKey{{keys[0].PublicKey()}},
		Signatures:   [][][]byte{{badSig.Marshal()}},
		Descriptions: []string{signing.UnknownSignature},
	}
	missingDescriptions := validSet.Copy()
	missingDescriptions.Descriptions = nil
	missingSignatures := validSet.Copy()
	missingSignatures.Signatures = nil
	nilPublicKey := validSet.Copy()
	nilPublicKey.PublicKeys[0][0] = nil
	tests := []struct {
		name          string
		message       string
		set           *ml_dsa_87.SignatureBatch
		preFilledSets []*ml_dsa_87.SignatureBatch
		want          pubsub.ValidationResult
	}{
		{
			name:    "empty queue",
			message: "random",
			set:     validSet,
			want:    pubsub.ValidationAccept,
		},
		{
			name:    "invalid set",
			message: "random",
			set:     invalidSet,
			want:    pubsub.ValidationReject,
		},
		{
			name:          "invalid set in routine with valid set",
			message:       "random",
			set:           validSet,
			preFilledSets: []*ml_dsa_87.SignatureBatch{invalidSet},
			want:          pubsub.ValidationAccept,
		},
		{
			name:          "valid set in routine with invalid set",
			message:       "random",
			set:           invalidSet,
			preFilledSets: []*ml_dsa_87.SignatureBatch{validSet},
			want:          pubsub.ValidationReject,
		},
		{
			name:    "missing descriptions falls back to verification",
			message: "random",
			set:     missingDescriptions,
			want:    pubsub.ValidationAccept,
		},
		{
			name:    "missing signature groups",
			message: "random",
			set:     missingSignatures,
			want:    pubsub.ValidationReject,
		},
		{
			name:    "nil public key",
			message: "random",
			set:     nilPublicKey,
			want:    pubsub.ValidationReject,
		},
		{
			name:          "malformed set in routine with valid set",
			message:       "random",
			set:           validSet,
			preFilledSets: []*ml_dsa_87.SignatureBatch{missingSignatures},
			want:          pubsub.ValidationAccept,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			svc := &Service{
				ctx:           ctx,
				cancel:        cancel,
				signatureChan: make(chan *signatureVerifier, verifierLimit),
			}
			go svc.verifierRoutine()
			for _, st := range tt.preFilledSets {
				svc.signatureChan <- &signatureVerifier{set: st.Copy(), resChan: make(chan error, 10)}
			}
			got, err := svc.validateWithBatchVerifier(context.Background(), tt.message, tt.set)
			if got != tt.want {
				t.Errorf("validateWithBatchVerifier() = %v, want %v", got, tt.want)
			}
			if err != nil && tt.want == pubsub.ValidationAccept {
				t.Errorf("Wanted no error but received: %v", err)
			}
			cancel()
		})
	}
}
