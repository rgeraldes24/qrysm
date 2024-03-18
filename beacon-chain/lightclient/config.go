package lightclient

import (
	"encoding/json"
	"strconv"

	"github.com/theQRL/go-zond/common/hexutil"
	"github.com/theQRL/qrysm/v4/config/params"
	types "github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
)

// ConfigJSON is the JSON representation of the light client config.
type ConfigJSON struct {
	GenesisForkVersion           string `json:"genesis_fork_version"             hex:"true"`
	MinSyncCommitteeParticipants string `json:"min_sync_committee_participants"`
	GenesisSlot                  string `json:"genesis_slot"`
	DomainSyncCommittee          string `json:"domain_sync_committee"            hex:"true"`
	SlotsPerEpoch                string `json:"slots_per_epoch"`
	EpochsPerSyncCommitteePeriod string `json:"epochs_per_sync_committee_period"`
	SecondsPerSlot               string `json:"seconds_per_slot"`
}

// Config is the light client configuration. It consists of the subset of the beacon chain configuration relevant to the
// light client. Unlike the beacon chain configuration it is serializable to JSON, hence it's a separate object.
type Config struct {
	GenesisForkVersion           []byte
	MinSyncCommitteeParticipants uint64
	GenesisSlot                  types.Slot
	DomainSyncCommittee          [4]byte
	SlotsPerEpoch                types.Slot
	EpochsPerSyncCommitteePeriod types.Epoch
	SecondsPerSlot               uint64
}

// NewConfig creates a new light client configuration from a beacon chain configuration.
func NewConfig(chainConfig *params.BeaconChainConfig) *Config {
	return &Config{
		GenesisForkVersion:           chainConfig.GenesisForkVersion,
		MinSyncCommitteeParticipants: chainConfig.MinSyncCommitteeParticipants,
		GenesisSlot:                  chainConfig.GenesisSlot,
		DomainSyncCommittee:          chainConfig.DomainSyncCommittee,
		SlotsPerEpoch:                chainConfig.SlotsPerEpoch,
		EpochsPerSyncCommitteePeriod: chainConfig.EpochsPerSyncCommitteePeriod,
		SecondsPerSlot:               chainConfig.SecondsPerSlot,
	}
}

func (c *Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(&ConfigJSON{
		GenesisForkVersion:           hexutil.Encode(c.GenesisForkVersion),
		MinSyncCommitteeParticipants: strconv.FormatUint(c.MinSyncCommitteeParticipants, 10),
		GenesisSlot:                  strconv.FormatUint(uint64(c.GenesisSlot), 10),
		DomainSyncCommittee:          hexutil.Encode(c.DomainSyncCommittee[:]),
		SlotsPerEpoch:                strconv.FormatUint(uint64(c.SlotsPerEpoch), 10),
		EpochsPerSyncCommitteePeriod: strconv.FormatUint(uint64(c.EpochsPerSyncCommitteePeriod), 10),
		SecondsPerSlot:               strconv.FormatUint(c.SecondsPerSlot, 10),
	})
}

func (c *Config) UnmarshalJSON(input []byte) error {
	var configJSON ConfigJSON
	if err := json.Unmarshal(input, &configJSON); err != nil {
		return err
	}
	var config Config
	var err error

	if config.GenesisForkVersion, err = hexutil.Decode(configJSON.GenesisForkVersion); err != nil {
		return err
	}
	if config.MinSyncCommitteeParticipants, err = strconv.ParseUint(configJSON.MinSyncCommitteeParticipants, 10, 64); err != nil {
		return err
	}
	genesisSlot, err := strconv.ParseUint(configJSON.GenesisSlot, 10, 64)
	if err != nil {
		return err
	}
	config.GenesisSlot = types.Slot(genesisSlot)
	domainSyncCommittee, err := hexutil.Decode(configJSON.DomainSyncCommittee)
	if err != nil {
		return err
	}
	config.DomainSyncCommittee = bytesutil.ToBytes4(domainSyncCommittee)
	slotsPerEpoch, err := strconv.ParseUint(configJSON.SlotsPerEpoch, 10, 64)
	if err != nil {
		return err
	}
	config.SlotsPerEpoch = types.Slot(slotsPerEpoch)
	epochsPerSyncCommitteePeriod, err := strconv.ParseUint(configJSON.EpochsPerSyncCommitteePeriod, 10, 64)
	if err != nil {
		return err
	}
	config.EpochsPerSyncCommitteePeriod = types.Epoch(epochsPerSyncCommitteePeriod)
	if config.SecondsPerSlot, err = strconv.ParseUint(configJSON.SecondsPerSlot, 10, 64); err != nil {
		return err
	}
	*c = config
	return nil
}
