package mock

import (
	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// PoolMock is a fake implementation of PoolManager.
type PoolMock struct {
	Changes []*qrysmpb.SignedDilithiumToExecutionChange
}

// PendingDilithiumToExecChanges --
func (m *PoolMock) PendingDilithiumToExecChanges() ([]*qrysmpb.SignedDilithiumToExecutionChange, error) {
	return m.Changes, nil
}

// DilithiumToExecChangesForInclusion --
func (m *PoolMock) DilithiumToExecChangesForInclusion(_ state.ReadOnlyBeaconState) ([]*qrysmpb.SignedDilithiumToExecutionChange, error) {
	return m.Changes, nil
}

// InsertDilithiumToExecChange --
func (m *PoolMock) InsertDilithiumToExecChange(change *qrysmpb.SignedDilithiumToExecutionChange) {
	m.Changes = append(m.Changes, change)
}

// MarkIncluded --
func (*PoolMock) MarkIncluded(_ *qrysmpb.SignedDilithiumToExecutionChange) {
	panic("implement me")
}

// ValidatorExists --
func (*PoolMock) ValidatorExists(_ primitives.ValidatorIndex) bool {
	panic("implement me")
}
