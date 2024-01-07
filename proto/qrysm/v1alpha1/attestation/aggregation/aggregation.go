// Package aggregation contains implementations of bitlist aggregation algorithms and heuristics.
package aggregation

import (
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/theQRL/go-bitfield"
)

var _ = logrus.WithField("prefix", "aggregation")

var (
	// ErrBitsOverlap is returned when two bitlists overlap with each other.
	ErrBitsOverlap = errors.New("overlapping aggregation bits")

	// ErrInvalidStrategy is returned when invalid aggregation strategy is selected.
	ErrInvalidStrategy = errors.New("invalid aggregation strategy")
)

// Aggregation defines the bitlist aggregation problem solution.
type Aggregation struct {
	// Coverage represents the best available solution to bitlist aggregation.
	Coverage bitfield.Bitlist
	// Keys is a list of candidate keys in the best available solution.
	Keys []int
}
