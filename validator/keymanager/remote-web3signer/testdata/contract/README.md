# Remote signer contract examples

Request and response bodies of the remote signer contract, version 1.0.0, transcribed as data from the examples in
go-remotesigner `api/openapi/consensus.yaml` at commit `2742eb73f5668a9a91d04ffe446e97c2975abae8`. They describe the
`end-to-end` network (minimal preset, `GENESIS_FORK_VERSION 0x000000fd`, 10 s slots, 6 slots per epoch).

- Public keys and signatures are fixed byte patterns (`0xa5` and `0x5a` repeated), not real keys.
- Signing roots are placeholders, except in `aggregation_slot.json`, so the tests check the request shape and every
  value the client maps, not the roots.
- `sync_committee_contribution_and_proof_mainnet.json` is the minimal example with the 16-byte mainnet bitvector, so
  both presets are covered.
- `capabilities_*.json` are the two `GET /api/v1/consensus/capabilities` examples (GVR pinned, and trust-on-first-use
  pending).

`v1/contract_test.go` builds the qrysm `SignRequest` that carries each example's values, runs the mapper and requires
the produced JSON to equal the example. The golden vectors of go-remotesigner milestone S3 (CC0 data with
`object_ssz_hex` and real roots) replace these files once they exist.
