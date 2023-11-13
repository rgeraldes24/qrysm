package state_native

import (
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// LatestExecutionPayloadHeader of the beacon state.
func (b *BeaconState) LatestExecutionPayloadHeader() (interfaces.ExecutionData, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	return blocks.WrappedExecutionPayloadHeader(b.latestExecutionPayloadHeaderVal(), 0)
}

// latestExecutionPayloadHeaderVal of the beacon state.
// This assumes that a lock is already held on BeaconState.
func (b *BeaconState) latestExecutionPayloadHeaderVal() *enginev1.ExecutionPayloadHeader {
	return zondpb.CopyExecutionPayloadHeader(b.latestExecutionPayloadHeader)
}
