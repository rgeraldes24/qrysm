//go:build tools

package main

// make e2e builds gqrl from the go-qrl version go.mod selects. Importing it
// here keeps `go mod tidy` from dropping the modules and go.sum entries it
// needs.
import (
	_ "github.com/theQRL/go-qrl/cmd/gqrl"
)
