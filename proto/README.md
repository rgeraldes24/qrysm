# Qrysm Protocol Buffers

This package defines common protobuf messages and services used by Qrysm. For now, we are checking in all generated code to support native go dependency management.

## Regenerating the generated code

After changing a `.proto` file, or the proto packages, SSZ targets or SSZ sizes in `build/gen/config.go`, regenerate the `.pb.go`, `.pb.gw.go` and `generated.ssz.go` files with:

```bash
make gen proto ssz
```

`make gen` with no arguments also regenerates the mocks. The files whose code differs between the mainnet and minimal configurations get a `//go:build !minimal` constraint and a `.minimal.` twin built with `-tags minimal`.
