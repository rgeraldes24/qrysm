# Third Party Package Patching

This directory holds local patches to third-party code used to build Qrysm.

## com_github_prysmaticlabs_fastssz-validation.patch

A patch to `sszgen`, the SSZ code generator of `github.com/prysmaticlabs/fastssz`. It makes the
generated SSZ code reject bitvector padding bits, encodings of 2^32 bytes or more and out-of-bounds
offsets, and validate bitlists. The generated `generated.ssz.go` files rely on it.

`make gen ssz` (see `build/gen/sszgen.go`) copies the fastssz version `go.mod` selects out of the
module cache, applies the patch with `git apply`, and builds `sszgen` from the patched copy. A hunk
that no longer applies fails the generation, and `go test ./build/gen` checks that the patch applies
to the pinned version.

## Creating or updating a patch

Make the change in a copy `b` of the original `a` and diff them with paths relative to the module
root (applied with `-p1`):

```
cd /tmp
git clone https://github.com/someteam/somerepo a
git clone https://github.com/someteam/somerepo b && cd b
# make the changes
cd ..
diff -ur --exclude=".git" a b > path/to/qrysm/third_party/YOURPATCH.patch
```
