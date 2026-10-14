// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package bazel

import (
	"path"
	"path/filepath"
	"testing"

	"github.com/theQRL/qrysm/testing/require"
)

// TestDataPath returns the absolute path to an asset in the testdata directory
// of the package under test.
//
// For example, if there is a file testdata/a.txt, you can get a path to that
// file using TestDataPath(t, "a.txt").
func TestDataPath(t testing.TB, relative ...string) string {
	relative = append([]string{"testdata"}, relative...)
	ret, err := filepath.Abs(path.Join(relative...))
	require.NoError(t, err)
	return ret
}
