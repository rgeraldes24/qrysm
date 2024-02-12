package dilithiumt

import (
	"errors"
	"fmt"
	"runtime"

	pkgerrors "github.com/pkg/errors"
	"github.com/theQRL/go-qrllib/dilithium"
	field_params "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/crypto/dilithium/common"
	"golang.org/x/sync/errgroup"
)

var ErrSignatureVerificationFailed = errors.New("signature verification failed")

// Signature used in the Dilithium signature scheme.
type Signature struct {
	s *[field_params.DilithiumSignatureLength]uint8
}

func SignatureFromBytes(sig []byte) (common.Signature, error) {
	if len(sig) != field_params.DilithiumSignatureLength {
		return nil, fmt.Errorf("signature must be %d bytes", field_params.DilithiumSignatureLength)
	}
	var signature [field_params.DilithiumSignatureLength]uint8
	copy(signature[:], sig)
	return &Signature{s: &signature}, nil
}

func MultipleSignaturesFromBytes(multiSigs [][]byte) ([]common.Signature, error) {
	if len(multiSigs) == 0 {
		return nil, fmt.Errorf("0 signatures provided to the method")
	}
	for _, s := range multiSigs {
		if len(s) != field_params.DilithiumSignatureLength {
			return nil, fmt.Errorf("signature must be %d bytes", field_params.DilithiumSignatureLength)
		}
	}
	wrappedSigs := make([]common.Signature, len(multiSigs))
	for i, signature := range multiSigs {
		var copiedSig [field_params.DilithiumSignatureLength]uint8
		copy(copiedSig[:], signature)
		wrappedSigs[i] = &Signature{s: &copiedSig}
	}
	return wrappedSigs, nil
}

func (s *Signature) Verify(pubKey common.PublicKey, msg []byte) bool {
	return dilithium.Verify(msg, *s.s, pubKey.(*PublicKey).p)
}

func VerifySignature(sig []byte, msg [32]byte, pubKey common.PublicKey) (bool, error) {
	rSig, err := SignatureFromBytes(sig)
	if err != nil {
		return false, err
	}
	return rSig.Verify(pubKey, msg[:]), nil
}

func VerifyMultipleSignatures(sigsBatches [][][]byte, msgs [][32]byte, pubKeysBatches [][]common.PublicKey) (bool, error) {
	var (
		lenSigsBatches    = len(sigsBatches)
		lenPubKeysBatches = len(pubKeysBatches)
	)

	if len(sigsBatches) == 0 || len(pubKeysBatches) == 0 {
		return false, nil
	}

	lenMsgsBatches := len(msgs)
	if lenSigsBatches != lenPubKeysBatches || lenSigsBatches != lenMsgsBatches {
		return false, pkgerrors.Errorf("provided signatures batches, pubkeys batches and messages have differing lengths. SB: %d, PB: %d, M: %d",
			lenSigsBatches, lenPubKeysBatches, lenMsgsBatches)
	}

	n := runtime.GOMAXPROCS(0) - 1
	grp := errgroup.Group{}
	grp.SetLimit(n)

	for i := 0; i < lenMsgsBatches; i++ {
		if len(sigsBatches[i]) != len(pubKeysBatches[i]) {
			return false, pkgerrors.Errorf("provided signatures, pubkeys have differing lengths. S: %d, P: %d, Batch: %d",
				len(sigsBatches[i]), len(pubKeysBatches[i]), i)
		}

		for j, sig := range sigsBatches[i] {
			sigCopy := make([]byte, len(sig))
			copy(sigCopy, sig)
			ic := i
			jc := j

			grp.Go(func() error {
				ok, err := VerifySignature(sigCopy, msgs[ic], pubKeysBatches[ic][jc])
				if err != nil {
					return err
				}
				if !ok {
					return ErrSignatureVerificationFailed
				}

				return nil
			})
		}
	}

	if err := grp.Wait(); err != nil {
		if !pkgerrors.Is(err, ErrSignatureVerificationFailed) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (s *Signature) Marshal() []byte {
	return s.s[:]
}

func (s *Signature) Copy() common.Signature {
	sign := *s.s
	return &Signature{s: &sign}
}
