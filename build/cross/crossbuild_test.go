package main

import (
	"reflect"
	"testing"
)

func TestTagFlag(t *testing.T) {
	c := Config{Tagflag: "-tags=develop", BinTags: ParseBinTags("qrysmctl=noMainnetGenesis other=a,b")}

	tests := map[string]string{
		"qrysmctl":     "-tags=develop,noMainnetGenesis",
		"other":        "-tags=develop,a,b",
		"beacon-chain": "-tags=develop",
	}
	for bin, want := range tests {
		if got := c.tagFlag(bin); got != want {
			t.Errorf("tagFlag(%q) = %q, want %q", bin, got, want)
		}
	}

	if got := (Config{}).tagFlag("beacon-chain"); got != "" {
		t.Errorf("tagFlag without tags = %q, want empty", got)
	}
}

func TestParseTargets(t *testing.T) {
	got := parseTargets("linux/amd64/x86_64-linux-gnu.2.31 bogus windows/amd64/x86_64-windows-gnu")
	want := []Target{{Linux, AMD64, "x86_64-linux-gnu.2.31"}, {Windows, AMD64, "x86_64-windows-gnu"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseTargets = %v, want %v", got, want)
	}
}

func TestArtifactName(t *testing.T) {
	if got := ArtifactName("validator", "v1.0.0", Target{OS: Windows, Arch: AMD64}); got != "validator-v1.0.0-windows-amd64.exe" {
		t.Errorf("ArtifactName = %q", got)
	}

	if got := ArtifactName("beacon-chain", "v1.0.0", Target{OS: Linux, Arch: ARM64}); got != "beacon-chain-v1.0.0-linux-arm64" {
		t.Errorf("ArtifactName = %q", got)
	}
}
