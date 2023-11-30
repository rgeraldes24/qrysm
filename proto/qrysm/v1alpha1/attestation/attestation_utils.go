// Package attestationutil contains useful helpers for converting
// attestations into indexed form.
package attestation

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"sort"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"
)

type signatureSlices struct {
	attestingIndices []uint64
	signatures       [][]byte
}

type sortByValidatorIdx signatureSlices

func (s sortByValidatorIdx) Len() int {
	return len(s.signatures)
}

func (s sortByValidatorIdx) Swap(i, j int) {
	s.attestingIndices[i], s.attestingIndices[j] = s.attestingIndices[j], s.attestingIndices[i]
	s.signatures[i], s.signatures[j] = s.signatures[j], s.signatures[i]
}

func (s sortByValidatorIdx) Less(i, j int) bool {
	return s.attestingIndices[i] < s.attestingIndices[j]
}

// ConvertToIndexed converts attestation to (almost) indexed-verifiable form.
func ConvertToIndexed(ctx context.Context, attestation *zondpb.Attestation, committee []primitives.ValidatorIndex) (*zondpb.IndexedAttestation, error) {
	attIndices, err := AttestingIndices(attestation.ParticipationBits, committee)
	if err != nil {
		return nil, err
	}

	sigSlices := signatureSlices{
		attestingIndices: attIndices,
		signatures:       attestation.Signatures,
	}
	sort.Sort(sortByValidatorIdx(sigSlices))

	return &zondpb.IndexedAttestation{
		Data:             attestation.Data,
		Signatures:       sigSlices.signatures,
		AttestingIndices: sigSlices.attestingIndices,
	}, nil
}

// NOTE(rgeraldes24) - AttestingIndices can be simplified since we already index the participation idx.
// The only issue is that it is not sorted and we need to double check if that's necessary.
// AttestingIndices returns the attesting participants indices from the attestation data.
func AttestingIndices(bf bitfield.Bitfield, committee []primitives.ValidatorIndex) ([]uint64, error) {
	if bf.Len() != uint64(len(committee)) {
		return nil, fmt.Errorf("bitfield length %d is not equal to committee length %d", bf.Len(), len(committee))
	}
	indices := make([]uint64, 0, bf.Count())
	for _, idx := range bf.BitIndices() {
		if idx < len(committee) {
			indices = append(indices, uint64(committee[idx]))
		}
	}
	return indices, nil
}

// NOTE(rgeraldes24): order of pubkeys is correct via VerifyIndexedAttestation.
// VerifyIndexedAttestationSigs this helper function performs the last part of the
// spec indexed attestation validation - signatures verification.
func VerifyIndexedAttestationSigs(ctx context.Context, indexedAtt *zondpb.IndexedAttestation, pubKeys []dilithium.PublicKey, domain []byte) error {
	ctx, span := trace.StartSpan(ctx, "attestationutil.VerifyIndexedAttestationSig")
	defer span.End()

	if len(indexedAtt.Signatures) != len(pubKeys) {
		return fmt.Errorf("signatures length %d is not equal to pub keys length %d", len(indexedAtt.Signatures), len(pubKeys))
	}

	messageHash, err := signing.ComputeSigningRoot(indexedAtt.Data, domain)
	if err != nil {
		return errors.Wrap(err, "could not get signing root of object")
	}

	n := runtime.GOMAXPROCS(0) - 1
	grp := errgroup.Group{}
	grp.SetLimit(n)
	for i, rawSig := range indexedAtt.Signatures {
		// move inside the code below?
		sig, err := dilithium.SignatureFromBytes(rawSig)
		if err != nil {
			return errors.Wrap(err, "could not convert bytes to signature")
		}

		iCopy := i
		grp.Go(func() error {
			if !sig.Verify(pubKeys[iCopy], messageHash[:]) {
				return signing.ErrSigFailedToVerify
			}

			return nil
		})
	}

	if err := grp.Wait(); err != nil {
		return err
	}

	return nil
}

// IsValidAttestationIndices this helper function performs the first part of the
// spec indexed attestation validation starting at Check if “indexed_attestation“
// comment and ends at Verify aggregate signature comment.
func IsValidAttestationIndices(ctx context.Context, indexedAttestation *zondpb.IndexedAttestation) error {
	ctx, span := trace.StartSpan(ctx, "attestationutil.IsValidAttestationIndices")
	defer span.End()

	if indexedAttestation == nil || indexedAttestation.Data == nil || indexedAttestation.Data.Target == nil || indexedAttestation.AttestingIndices == nil {
		return errors.New("nil or missing indexed attestation data")
	}
	indices := indexedAttestation.AttestingIndices
	if len(indices) == 0 {
		return errors.New("expected non-empty attesting indices")
	}
	if uint64(len(indices)) > params.BeaconConfig().MaxValidatorsPerCommittee {
		return fmt.Errorf("validator indices count exceeds MAX_VALIDATORS_PER_COMMITTEE, %d > %d", len(indices), params.BeaconConfig().MaxValidatorsPerCommittee)
	}
	for i := 1; i < len(indices); i++ {
		if indices[i-1] >= indices[i] {
			return errors.New("attesting indices is not uniquely sorted")
		}
	}
	return nil
}

// AttDataIsEqual this function performs an equality check between 2 attestation data, if they're unequal, it will return false.
func AttDataIsEqual(attData1, attData2 *zondpb.AttestationData) bool {
	if attData1.Slot != attData2.Slot {
		return false
	}
	if attData1.CommitteeIndex != attData2.CommitteeIndex {
		return false
	}
	if !bytes.Equal(attData1.BeaconBlockRoot, attData2.BeaconBlockRoot) {
		return false
	}
	if attData1.Source.Epoch != attData2.Source.Epoch {
		return false
	}
	if !bytes.Equal(attData1.Source.Root, attData2.Source.Root) {
		return false
	}
	if attData1.Target.Epoch != attData2.Target.Epoch {
		return false
	}
	if !bytes.Equal(attData1.Target.Root, attData2.Target.Root) {
		return false
	}
	return true
}

// CheckPointIsEqual performs an equality check between 2 check points, returns false if unequal.
func CheckPointIsEqual(checkPt1, checkPt2 *zondpb.Checkpoint) bool {
	if checkPt1.Epoch != checkPt2.Epoch {
		return false
	}
	if !bytes.Equal(checkPt1.Root, checkPt2.Root) {
		return false
	}
	return true
}

func NewBits(baseField bitfield.Bitfield, newField bitfield.Bitfield) []int {
	baseFieldBytes := baseField.Bytes()
	if len(baseFieldBytes) == 0 {
		return newField.BitIndices()
	}

	newFieldBytes := newField.Bytes()
	newBits := make([]int, 0, newField.Count())

	for i := 0; i < len(baseFieldBytes); i++ {
		// start checking the byte and move to bits if a new participant is found
		if baseFieldBytes[i]^(baseFieldBytes[i]|newFieldBytes[i]) != 0 {
			var bitIdx int = i * 8
			for j := 0; j < 8; j, bitIdx = j+1, bitIdx+1 {
				// base field bit must be set to zero and the new field bit must be set to one
				if !baseField.BitAt(uint64(bitIdx)) && newField.BitAt(uint64(bitIdx)) {
					newBits = append(newBits, bitIdx)
				}
			}
		}
	}
	return newBits
}

func SearchInsertIdxWithOffset(arr []int, initialIdx int, target int) (int, error) {
	arrLen := len(arr)

	if arrLen == 0 {
		return 0, nil
	}

	if initialIdx > (arrLen - 1) {
		return 0, fmt.Errorf("Invalid initial index %d for slice length %d", initialIdx, arrLen)
	}

	if target <= arr[initialIdx] {
		return initialIdx, nil
	}

	if target > arr[arrLen-1] {
		return arrLen, nil
	}

	low := initialIdx
	high := arrLen - 1

	for low <= high {
		mid := (low + high) / 2
		if arr[mid] == target {
			return mid + 1, nil
		}
		if arr[mid] > target {
			high = mid - 1
		} else {
			low = mid + 1
		}
	}

	return low, nil
}
