package main

import (
	"slices"
	"testing"
)

func TestSelectTargets(t *testing.T) {
	tests := []struct {
		args    []string
		want    []kind
		wantErr bool
	}{
		{args: nil, want: suites[suitePresubmit]},
		{args: []string{"minimal"}, want: []kind{kindMinimal}},
		{args: []string{"presubmit", "minimal", "builder"}, want: []kind{kindMinimal, kindSlashing, kindSlasher, kindBuilder}},
		{args: []string{"nope"}, wantErr: true},
	}

	for _, tt := range tests {
		_, got, err := selectTargets(tt.args)
		if (err != nil) != tt.wantErr {
			t.Fatalf("selectTargets(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
		}

		if !slices.Equal(got, tt.want) {
			t.Errorf("selectTargets(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

func TestSuitesOnlyReferenceKnownKinds(t *testing.T) {
	for s, members := range suites {
		for _, k := range members {
			if kinds[k].run == "" {
				t.Errorf("suite %s references unknown kind %s", s, k)
			}
		}
	}
}
