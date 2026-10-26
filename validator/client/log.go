package client

import (
	"fmt"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/validator/keymanager"
)

var log = logrus.WithField("prefix", "validator")

// logSigningFailure logs a failed keymanager signing or RANDAO reveal call. A
// remote signer's refusal reason is attached when the error carries one. A
// protection refusal (keymanager.ErrSigningRefused) is logged at WARN, any
// other failure at ERROR.
func logSigningFailure(entry *logrus.Entry, err error, msg string) {
	entry = entry.WithError(err)
	var r keymanager.Reasoner
	if errors.As(err, &r) {
		entry = entry.WithField("reason", r.Reason())
	}
	if errors.Is(err, keymanager.ErrSigningRefused) {
		entry.Warn(msg)
		return
	}
	entry.Error(msg)
}

type attSubmitted struct {
	data              *qrysmpb.AttestationData
	attesterIndices   []primitives.ValidatorIndex
	aggregatorIndices []primitives.ValidatorIndex
}

// LogAttestationsSubmitted logs info about submitted attestations.
func (v *validator) LogAttestationsSubmitted() {
	v.attLogsLock.Lock()
	defer v.attLogsLock.Unlock()

	for _, attLog := range v.attLogs {
		log.WithFields(logrus.Fields{
			"Slot":              attLog.data.Slot,
			"CommitteeIndex":    attLog.data.CommitteeIndex,
			"BeaconBlockRoot":   fmt.Sprintf("%#x", bytesutil.Trunc(attLog.data.BeaconBlockRoot)),
			"SourceEpoch":       attLog.data.Source.Epoch,
			"SourceRoot":        fmt.Sprintf("%#x", bytesutil.Trunc(attLog.data.Source.Root)),
			"TargetEpoch":       attLog.data.Target.Epoch,
			"TargetRoot":        fmt.Sprintf("%#x", bytesutil.Trunc(attLog.data.Target.Root)),
			"AttesterIndices":   attLog.attesterIndices,
			"AggregatorIndices": attLog.aggregatorIndices,
		}).Info("Submitted new attestations")
	}

	v.attLogs = make(map[[32]byte]*attSubmitted)
}

// LogSyncCommitteeMessagesSubmitted logs info about submitted sync committee messages.
func (v *validator) LogSyncCommitteeMessagesSubmitted() {
	log.WithField("messages", v.syncCommitteeStats.totalMessagesSubmitted).Debug("Submitted sync committee messages successfully to beacon node")
	// Reset the amount.
	atomic.StoreUint64(&v.syncCommitteeStats.totalMessagesSubmitted, 0)
}
