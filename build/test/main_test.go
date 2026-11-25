package main

import (
	"slices"
	"testing"
)

func TestSelectKinds(t *testing.T) {
	tests := []struct {
		args    []string
		want    []kind
		race    bool
		wantErr bool
	}{
		{want: validKinds},
		{race: true, want: []kind{mainnet, mainnetSpectest}},
		{args: []string{"minimal", "mainnet", "minimal"}, want: []kind{minimal, mainnet}},
		{args: []string{"nope"}, wantErr: true},
	}

	for _, tt := range tests {
		got, err := selectKinds(tt.args, tt.race)
		if (err != nil) != tt.wantErr {
			t.Fatalf("selectKinds(%v, %v) error = %v", tt.args, tt.race, err)
		}

		if !slices.Equal(got, tt.want) {
			t.Errorf("selectKinds(%v, %v) = %v, want %v", tt.args, tt.race, got, tt.want)
		}
	}
}

func TestExcludeRe(t *testing.T) {
	for pkg, excluded := range map[string]bool{
		"github.com/theQRL/qrysm/testing/endtoend":                       true,
		"github.com/theQRL/qrysm/testing/endtoend/components":            true,
		"github.com/theQRL/qrysm/testing/spectest/mainnet/zond/sanity":   true,
		"github.com/theQRL/qrysm/beacon-chain/rpc/qrysm/v1alpha1/beacon": true,
		// Mainnet tests of mixed packages run in the mainnet pass.
		"github.com/theQRL/qrysm/beacon-chain/rpc/qrysm/v1alpha1/validator": false,
		"github.com/theQRL/qrysm/beacon-chain/core/helpers":                 false,
		"github.com/theQRL/qrysm/testing/spectest/utils":                    true,
	} {
		if got := excludeRe.MatchString(pkg); got != excluded {
			t.Errorf("excludeRe.MatchString(%q) = %v, want %v", pkg, got, excluded)
		}
	}
}
