// Command cross cross-compiles the release binaries (usually inside the
// builder container, through `make dist`).
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := ConfigFromEnv().Build(); err != nil {
		fmt.Fprintln(os.Stderr, "❌ cross:", err)
		os.Exit(1)
	}
}
