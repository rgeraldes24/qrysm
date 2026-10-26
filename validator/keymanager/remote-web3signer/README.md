# Remote signer keymanager

A keymanager that keeps validator keys outside the validator client and asks a remote signer to sign. The signer
implements the Web3Signer-style remote signing API adapted to QRL (ML-DSA-87 keys, per-participant signature lists,
`Q` addresses, a RANDAO hash-onion reveal endpoint). The contract, version 1.0.0, is published by go-remotesigner
(`api/openapi/consensus.yaml` and `docs/contract.md`).

## Validator client flags

`--validators-external-signer-url=http(s)://host:port` selects this keymanager. The validator client then opens no
wallet; it waits for chain start, reads the genesis validators root from its database, checks the signer's
capabilities against the chain and stops with an error naming each differing field if they do not match.

The keys to validate with come from exactly one of:

- `--validators-external-signer-key-file`: one `0x` public key per line, blank lines and `#` comments allowed,
  duplicates rejected. The file is re-read when it changes (also when it is replaced by a rename); an invalid
  version is logged and the current keys are kept. Use this for more than a handful of keys. The remote keys API
  (below) can add and remove keys in it.
- `--validators-external-signer-public-keys`: a comma list of `0x` public keys. An ML-DSA-87 public key takes 5,186
  characters, so a list of more than about 25 keys exceeds the operating system's per-argument limit (`MAX_ARG_STRLEN`,
  128 KiB on Linux) and is **not supported**: use the key file.
- `--validators-external-signer-public-keys=<url>`, normally `http(s)://host:port/api/v1/consensus/publicKeys`: every
  key the signer holds. The request carries the bearer token, so with an `https` signer URL this URL must use `https`
  too. Never point two validator clients at the same signer this way.

`--validators-external-signer-auth-token-file` holds the sign-scope bearer token.
`--validators-external-signer-tls-ca-file`, `--validators-external-signer-tls-client-cert-file` and
`--validators-external-signer-tls-client-key-file` configure TLS and mutual TLS for an `https` URL.
`--validators-external-signer-timeout` (default 10s) bounds each request.

Slashing protection database: in remote mode the validator client keeps `validator.db` in `--wallet-dir` itself,
while a local wallet keeps it in `<wallet-dir>/direct`. When moving a local validator to a remote signer, pass
`--datadir=<wallet-dir>/direct` to keep its history. Without `--datadir`, the validator client refuses to start if
`<wallet-dir>/direct/validator.db` exists and the database it would use is missing or holds no signing history.

## Remote keys API

With `--rpc`, the validator client serves the keymanager API's remote keys endpoints at `/qrl/v1/remotekeys`, with
the same bearer token as the other keymanager endpoints:

- `GET` lists the validating keys, each with the signer URL without user information, and `readonly: true` unless
  the keys come from the key file.
- `POST {"remote_keys": [{"pubkey": "0x…", "url": "…"}]}` adds keys to the key file and
  `DELETE {"pubkeys": ["0x…"]}` removes their lines. Both answer one status per key: `imported`, `duplicate` or
  `error`, and `deleted`, `not_found` or `error`.

Only keys that come from `--validators-external-signer-key-file` can be added or deleted. The key file is read again
before each change, so that an edit the watcher has not reloaded yet is kept, and an invalid file is left alone. The
new file goes to a temporary file in the same directory, which is synced and renamed over the old one: a failed write
leaves the old file, and a reader never sees part of a file. Comments and the file's permissions are kept, and a
symbolic link is followed. The directory must be writable by the validator client. The new file keeps the old one's
group when the validator client is a member of it, but its owner only when the validator client runs as root: a key
file owned by another user becomes the validator client user's at the first change. Keys given as a list or by a
public keys URL cannot be changed, since a change held only in memory would be lost on restart: the validator client
could then sign again with a key that was deleted and moved to another validator client.

With a key file, the keymanager signs only for the keys in it, so a key that is deleted, or removed from the file by
an edit, gets no sign or RANDAO reveal request once the change is made. The validator client still keeps the key's
duties until the next epoch boundary, up to 128 slots of 60 seconds (about 2 hours 8 minutes) on mainnet, and logs an
error for each duty it can no longer sign. A request sent to the signer before the change is not recalled: move the
key to another validator client only after `DELETE` has answered.

The `url` of an imported key is ignored and never requested: every key is signed for by the signer that
`--validators-external-signer-url` names.

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
`keymanager.ErrSigningRefused`, which the validator client's duty code logs at WARN rather than ERROR. A 404 without
a `reason`, the answer to a path that matches no route, is reported as a wrong signer URL rather than a missing key.
A voluntary exit refused with 403 `ARTIFACT_TYPE_DISABLED` matches `keymanager.ErrVoluntaryExitSigningDisabled`, and
its message names the signer option `--voluntary-exit-signing-enabled`.
