# Remote signer keymanager

A keymanager that keeps validator keys outside the validator client and asks a remote signer to sign. The signer
implements the Web3Signer-style remote signing API adapted to QRL (ML-DSA-87 keys, per-participant signature lists,
`Q` addresses, a RANDAO hash-onion reveal endpoint). The contract, version 1.0.0, is published by go-remotesigner
(`api/openapi/consensus.yaml` and `docs/contract.md`).

The flags that select this keymanager are wired separately; this package is the client.

## API

- `POST /api/v1/consensus/sign/{pubkey}`: signs one artifact. The returned signature is verified against the
  public key and signing root before use. qrysm `SignRequest` oneofs map to contract types as follows:
    - `block_zond`, `blinded_block_zond` → `BLOCK_V2` (`version: "ZOND"`, block header only)
    - `attestation_data` → `ATTESTATION`
    - `aggregate_attestation_and_proof` → `AGGREGATE_AND_PROOF`
    - `slot` → `AGGREGATION_SLOT`
    - `exit` → `VOLUNTARY_EXIT`
    - `sync_message_block_root` → `SYNC_COMMITTEE_MESSAGE`
    - `sync_aggregator_selection_data` → `SYNC_COMMITTEE_SELECTION_PROOF`
    - `contribution_and_proof` → `SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF`
    - `registration` → `VALIDATOR_REGISTRATION` (no `fork_info`)
    - `epoch` → refused: RANDAO is a hash-onion reveal, not a signature
- `POST /api/v1/consensus/randao_reveal/{pubkey}`: returns the pre-image of the validator's RANDAO commitment for a
  proposal slot. `RANDAO_UNSUPPORTED_FOR_KEY` maps to `keymanager.ErrRandaoRevealUnsupported`; 429 and 503 are
  retried once within the slot.
- `GET /api/v1/consensus/capabilities`: `CheckCapabilities` compares the signer's network binding (fork version,
  genesis validators root, genesis time, slot timing), RANDAO support and contract major version with the validator's
  chain, and fails on any difference.
- `GET /api/v1/consensus/publicKeys`: the key list for the URL form of the public keys option. Every key it returns
  is validated, so two validator clients must not share it.
- `GET /upcheck`: plain-text `OK` once the signer is ready.

Requests carry `Accept: application/json` and, when configured, a sign-scope bearer token; an https URL requires TLS
1.3 and can use a custom CA and a client certificate. Error responses become `SignerError` values that carry the
signer's `reason` and unwrap to an error per status code. A 412 protection refusal also matches
`keymanager.ErrSigningRefused`, which the validator client's duty code logs at WARN rather than ERROR.
