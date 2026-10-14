package p2p

import (
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/testing/assert"
)

func TestOverlayParameters(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	pms := pubsubGossipParam()
	assert.Equal(t, gossipSubD, pms.D, "gossipSubD")
	assert.Equal(t, gossipSubDlo, pms.Dlo, "gossipSubDlo")
	assert.Equal(t, gossipSubDhi, pms.Dhi, "gossipSubDhi")
}

func TestGossipParameters(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	setPubSubParameters()
	pms := pubsubGossipParam()
	assert.Equal(t, gossipSubMcacheLen, pms.HistoryLength, "gossipSubMcacheLen")
	assert.Equal(t, gossipSubMcacheGossip, pms.HistoryGossip, "gossipSubMcacheGossip")
	cfg := params.BeaconConfig()
	epochDuration := time.Duration(uint64(cfg.SlotsPerEpoch)*cfg.SecondsPerSlot) * time.Second
	assert.Equal(t, epochDuration, pubsub.TimeCacheDuration, "gossipSubSeenTtl")
	// Message IDs must outlive the window in which attestations may still be propagated.
	propagationWindow := time.Duration(uint64(params.BeaconNetworkConfig().AttestationPropagationSlotRange)*cfg.SecondsPerSlot) * time.Second
	assert.Equal(t, true, pubsub.TimeCacheDuration >= propagationWindow, "seen TTL shorter than the attestation propagation window")
}

func TestFanoutParameters(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	pms := pubsubGossipParam()
	if pms.FanoutTTL != gossipSubFanoutTTL {
		t.Errorf("gossipSubFanoutTTL, wanted: %d, got: %d", gossipSubFanoutTTL, pms.FanoutTTL)
	}
}

func TestHeartbeatParameters(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	pms := pubsubGossipParam()
	if pms.HeartbeatInterval != gossipSubHeartbeatInterval {
		t.Errorf("gossipSubHeartbeatInterval, wanted: %d, got: %d", gossipSubHeartbeatInterval, pms.HeartbeatInterval)
	}
}

func TestMiscParameters(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	setPubSubParameters()
	assert.Equal(t, rSubD, 8, "rSubD")
}
