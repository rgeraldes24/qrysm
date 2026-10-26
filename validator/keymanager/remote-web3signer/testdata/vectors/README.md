# Remote signer golden vectors

Golden vectors of the remote signer contract, version 1.0.0 (`docs/contract.md` of
[go-remotesigner](https://github.com/rgeraldes24/go-remotesigner)), copied unchanged from its `testdata/vectors`.
go-remotesigner generates them with qrysm as the oracle; every file names the generator and the module versions it
used in its `generator` object. The format is described in go-remotesigner's `testdata/vectors/README.md`.

Only the files the client tests read are copied:

| Path | Git object id |
|---|---|
| `signing/` (tree) | `6291283ef905d3ef0517e10a31ee12cfdef7c19c` |
| `capabilities.json` (blob) | `88457b0503bb56f55f2fda74d2aa235f153e0a04` |

They were copied from go-remotesigner commit `8447c2d29add5f52b5157038bc6b74bd0ecef36b`. The ids are content
addresses, so any go-remotesigner commit whose `testdata/vectors/signing` and `testdata/vectors/capabilities.json`
have these ids holds the same files. To update the copy, replace the two paths with those of a newer commit and check
that both commands print the same ids:

```
git -C <go-remotesigner> rev-parse <commit>:testdata/vectors/signing <commit>:testdata/vectors/capabilities.json
git rev-parse HEAD:validator/keymanager/remote-web3signer/testdata/vectors/signing \
  HEAD:validator/keymanager/remote-web3signer/testdata/vectors/capabilities.json
```

## Licence

The files are data produced by a program. go-remotesigner dedicates them to the public domain under
[CC0-1.0](https://creativecommons.org/publicdomain/zero/1.0/), as the `description` of each file states.

## Tests

- `v1/contract_test.go` decodes the `object_ssz_hex` of every signing record with qrysm's `UnmarshalSSZ`, requires
  qrysm's object root, domain and signing root to equal the record's, builds the `SignRequest` the validator client
  builds, runs the mapper and requires the JSON to equal `request_json`. Only the letter case of `fee_recipient` may
  differ: go-qrl writes the QIP-55 mixed-case form, `request_json` is lower case, and the contract accepts either.
  - The minimal-preset contribution records hold the mainnet encoding, with the bitvector padded to 16 bytes
    (`authoritative: false`). The test cuts the bitvector to the 2 bytes a minimal build holds.
  - The mapper takes a block, which only the mainnet `real_block` record carries, in full and blinded form. For the
    other `BLOCK_V2` records, which hold only the header, the test maps a block with the header's slot, proposer and
    parent and state roots, and expects that block's body root.
  - The test needs a mainnet-preset build, because `object_ssz_hex` holds mainnet SSZ encodings; it is skipped
    otherwise.
- `keymanager_test.go` runs `CheckCapabilities` on every handshake of `capabilities.json`, and
  `internal/client_test.go` decodes every response in it.
