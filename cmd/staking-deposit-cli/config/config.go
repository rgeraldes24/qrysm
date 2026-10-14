package config

import (
	"github.com/theQRL/qrysm/cmd/staking-deposit-cli/misc"
)

const (
	TESTNET = "testnet"
	BETANET = "betanet"
	MAINNET = "mainnet"
	DEV     = "dev"
)

type Config struct {
	ChainSettings map[string]*ChainSetting
}

type ChainSetting struct {
	Name               string
	GenesisForkVersion []byte
}

func ToHex(data string) []byte {
	return misc.DecodeHex(data)
}

func GetConfig() *Config {
	c := &Config{
		ChainSettings: map[string]*ChainSetting{
			TESTNET: {
				Name:               TESTNET,
				GenesisForkVersion: ToHex("0x20000089"),
			},
			BETANET: {
				Name:               BETANET,
				GenesisForkVersion: ToHex("0x20000089"),
			},
			MAINNET: {
				Name:               MAINNET,
				GenesisForkVersion: ToHex("0x00000000"),
			},
			DEV: {
				Name:               DEV,
				GenesisForkVersion: ToHex("0x10000038"),
			},
		},
	}
	return c
}
