package dilithiumt

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"

	pkgerrors "github.com/pkg/errors"
	"github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/qrysm/v4/crypto/dilithium/common"
	"golang.org/x/sync/errgroup"
)

var ErrSignatureVerificationFailed = errors.New("signature verification failed")

// Signature used in the BLS signature scheme.
type Signature struct {
	s *[dilithium.CryptoBytes]uint8
}

func SignatureFromBytes(sig []byte) (common.Signature, error) {
	if len(sig) != dilithium.CryptoBytes {
		return nil, fmt.Errorf("signature must be %d bytes", dilithium.CryptoBytes)
	}
	var signature [dilithium.CryptoBytes]uint8
	copy(signature[:], sig)
	return &Signature{s: &signature}, nil
}

func AggregateCompressedSignatures(multiSigs [][]byte) (common.Signature, error) {
	panic("AggregateCompressedSignatures not supported for dilithium")
}

func MultipleSignaturesFromBytes(multiSigs [][]byte) ([]common.Signature, error) {
	if len(multiSigs) == 0 {
		return nil, fmt.Errorf("0 signatures provided to the method")
	}
	for _, s := range multiSigs {
		if len(s) != dilithium.CryptoBytes {
			return nil, fmt.Errorf("signature must be %d bytes", dilithium.CryptoBytes)
		}
	}
	wrappedSigs := make([]common.Signature, len(multiSigs))
	for i, signature := range multiSigs {
		var copiedSig [dilithium.CryptoBytes]uint8
		copy(copiedSig[:], signature)
		wrappedSigs[i] = &Signature{s: &copiedSig}
	}
	return wrappedSigs, nil
}

func (s *Signature) Verify(pubKey common.PublicKey, msg []byte) bool {
	return dilithium.Verify(msg, *s.s, pubKey.(*PublicKey).p)
}

func (s *Signature) AggregateVerify(pubKeys []common.PublicKey, msgs [][32]byte) bool {
	panic("AggregateVerify not supported for dilithium")
}

func (s *Signature) FastAggregateVerify(pubKeys []common.PublicKey, msg [32]byte) bool {
	panic("FastAggregateVerify not supported for dilithium")
}

func (s *Signature) Eth2FastAggregateVerify(pubKeys []common.PublicKey, msg [32]byte) bool {
	if len(pubKeys) == 0 && bytes.Equal(s.Marshal(), common.InfiniteSignature[:]) {
		return true
	}
	panic("Eth2FastAggregateVerify not supported for dilithium")
}

func NewAggregateSignature() common.Signature {
	panic("NewAggregateSignature not supported for dilithium")
}

func AggregateSignatures(sigs []common.Signature) common.Signature {
	panic("AggregateSignatures not supported for dilithium")
}

func UnaggregatedSignatures(sigs []common.Signature) [][]byte {
	if len(sigs) == 0 {
		return nil
	}

	unaggregatedSigns := make([][]byte, len(sigs))
	for i, sig := range sigs {
		copy(unaggregatedSigns[i], sig.Marshal())
	}

	return unaggregatedSigns
}

func VerifySignature(sig []byte, msg [32]byte, pubKey common.PublicKey) (bool, error) {
	rSig, err := SignatureFromBytes(sig)
	if err != nil {
		return false, err
	}
	return rSig.Verify(pubKey, msg[:]), nil
}

func VerifyMultipleSignatures(sigsBatches [][][]byte, msgsBatches [][32]byte, pubKeysBatches [][]common.PublicKey) (bool, error) {
	var (
		lenSigsBatches    = len(sigsBatches)
		lenMsgsBatches    = len(msgsBatches)
		lenPubKeysBatches = len(pubKeysBatches)
	)
	if lenSigsBatches == 0 || lenMsgsBatches == 0 || lenPubKeysBatches == 0 {
		return false, nil
	}

	if lenSigsBatches != lenPubKeysBatches || lenSigsBatches != lenMsgsBatches {
		return false, pkgerrors.Errorf("provided signatures, pubkeys and messages batches have differing lengths. S: %d, P: %d, M: %d",
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
			iCopy := i
			jCopy := j

			grp.Go(func() error {
				ok, err := VerifySignature(sigCopy, msgsBatches[iCopy], pubKeysBatches[iCopy][jCopy])
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
