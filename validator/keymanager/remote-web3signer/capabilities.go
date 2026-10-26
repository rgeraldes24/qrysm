package remote_web3signer

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/internal"
)

// SupportedAPIVersionMajor is the major version of the remote signer contract
// this client speaks.
const SupportedAPIVersionMajor = 1

// ErrCapabilitiesMismatch is returned by CheckCapabilities when the remote
// signer is bound to another chain or cannot serve this validator. The error
// text names every field that differs.
var ErrCapabilitiesMismatch = errors.New("remote signer capabilities do not match this validator")

// CheckCapabilities fetches the remote signer's capabilities and compares them
// with this validator's chain: the genesis fork version and slot timing from
// the beacon config, and the given genesis validators root and genesis time
// (unix seconds) learned at chain start. It fails when any of them differ,
// when the signer cannot release RANDAO reveals, or when the contract major
// version is not supported. A genesis validators root the signer has not
// pinned yet is only logged.
func (km *Keymanager) CheckCapabilities(ctx context.Context, genesisValidatorsRoot []byte, genesisTime uint64) error {
	if !bytesutil.IsValidRoot(genesisValidatorsRoot) {
		return fmt.Errorf("invalid genesis validators root %#x", genesisValidatorsRoot)
	}
	caps, err := km.client.Capabilities(ctx)
	if err != nil {
		return errors.Wrap(err, "could not fetch remote signer capabilities")
	}
	return checkCapabilities(caps, params.BeaconConfig(), genesisValidatorsRoot, genesisTime)
}

func checkCapabilities(
	caps *internal.Capabilities, cfg *params.BeaconChainConfig, genesisValidatorsRoot []byte, genesisTime uint64,
) error {
	var mismatches []string
	mismatch := func(format string, args ...any) {
		mismatches = append(mismatches, fmt.Sprintf(format, args...))
	}

	if major, ok := semverMajor(caps.APIVersion); !ok {
		mismatch("api_version: signer reports %q, not a semantic version", caps.APIVersion)
	} else if major != SupportedAPIVersionMajor {
		mismatch("api_version: signer %s, supported major version %d", caps.APIVersion, SupportedAPIVersionMajor)
	}

	n := caps.Network
	if !bytes.Equal(n.GenesisForkVersion, cfg.GenesisForkVersion) {
		mismatch("genesis_fork_version: signer %#x, validator %#x", []byte(n.GenesisForkVersion), cfg.GenesisForkVersion)
	}
	if n.GenesisValidatorsRoot == nil {
		log.Warn("Remote signer has not pinned a genesis validators root yet; it will trust the root of its first request")
	} else if !bytes.Equal(*n.GenesisValidatorsRoot, genesisValidatorsRoot) {
		mismatch("genesis_validators_root: signer %#x, validator %#x", []byte(*n.GenesisValidatorsRoot), genesisValidatorsRoot)
	}
	if n.GenesisTime == nil {
		mismatch("genesis_time: unknown to the signer, validator %d", genesisTime)
	} else if v, ok := parseUint64(*n.GenesisTime); !ok || v != genesisTime {
		mismatch("genesis_time: signer %s, validator %d", *n.GenesisTime, genesisTime)
	}
	if v, ok := parseUint64(n.SecondsPerSlot); !ok || v != cfg.SecondsPerSlot {
		mismatch("seconds_per_slot: signer %s, validator %d", n.SecondsPerSlot, cfg.SecondsPerSlot)
	}
	if v, ok := parseUint64(n.SlotsPerEpoch); !ok || v != uint64(cfg.SlotsPerEpoch) {
		mismatch("slots_per_epoch: signer %s, validator %d", n.SlotsPerEpoch, cfg.SlotsPerEpoch)
	}
	if !caps.RandaoReveal {
		mismatch("randao_reveal: the signer cannot release RANDAO reveals")
	}

	if len(mismatches) > 0 {
		return fmt.Errorf("%w: %s", ErrCapabilitiesMismatch, strings.Join(mismatches, "; "))
	}
	log.WithFields(logrus.Fields{
		"apiVersion":         caps.APIVersion,
		"network":            n.ConfigName,
		"selectionProofMode": caps.SelectionProofMode,
		"slashingProtection": caps.SlashingProtection,
	}).Info("Remote signer capabilities match this validator")
	return nil
}

// semverMajor returns the major version of a MAJOR.MINOR.PATCH string.
func semverMajor(v string) (uint64, bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return 0, false
	}
	var major uint64
	for i, p := range parts {
		n, ok := parseUint64(p)
		if !ok {
			return 0, false
		}
		if i == 0 {
			major = n
		}
	}
	return major, true
}

// parseUint64 parses a canonical decimal uint64: no sign and no leading zeros.
func parseUint64(s string) (uint64, bool) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil || strconv.FormatUint(v, 10) != s {
		return 0, false
	}
	return v, true
}
