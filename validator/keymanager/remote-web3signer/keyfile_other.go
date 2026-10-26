//go:build !unix

package remote_web3signer

import (
	"io/fs"
	"os"
)

// keepOwner leaves tmp as created: file ownership is not kept outside Unix.
func keepOwner(*os.File, fs.FileInfo) error {
	return nil
}
