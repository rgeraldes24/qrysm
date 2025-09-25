package mock

import (
	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// PoolMock is a fake implementation of PoolManager.
type PoolMock struct {
	Changes []*qrysmpb.SignedMLDSA87ToExecutionChange
}

// PendingMLDSA87ToExecChanges --
func (m *PoolMock) PendingMLDSA87ToExecChanges() ([]*qrysmpb.SignedMLDSA87ToExecutionChange, error) {
	return m.Changes, nil
}

// MLDSA87ToExecChangesForInclusion --
func (m *PoolMock) MLDSA87ToExecChangesForInclusion(_ state.ReadOnlyBeaconState) ([]*qrysmpb.SignedMLDSA87ToExecutionChange, error) {
	return m.Changes, nil
}

// InsertMLDSA87ToExecChange --
func (m *PoolMock) InsertMLDSA87ToExecChange(change *qrysmpb.SignedMLDSA87ToExecutionChange) {
	m.Changes = append(m.Changes, change)
}

// MarkIncluded --
func (*PoolMock) MarkIncluded(_ *qrysmpb.SignedMLDSA87ToExecutionChange) {
	panic("implement me")
}

// ValidatorExists --
func (*PoolMock) ValidatorExists(_ primitives.ValidatorIndex) bool {
	panic("implement me")
}
