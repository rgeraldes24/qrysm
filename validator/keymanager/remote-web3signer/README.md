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
`--datadir=<wallet-dir>/direct` to keep its history (see [The database directory](#the-database-directory)).

## Voluntary exits

`validator accounts voluntary-exit` and `qrysmctl validator exit` sign through the remote signer when given
`--validators-external-signer-url` and the validator client's other remote signer flags: the key file or public keys,
the auth token file, the TLS files and the timeout. They read the genesis validators root from the beacon node at
`--beacon-rpc-provider`, have the signer sign each exit and submit it to that beacon node, or with
`--voluntary-exit-json-output-path` write it to a file instead:

```
validator accounts voluntary-exit \
  --validators-external-signer-url=https://signer.internal:9000 \
  --validators-external-signer-key-file=/etc/qrysm/remote-keys.txt \
  --validators-external-signer-auth-token-file=/etc/qrysm/signer-auth-token \
  --validators-external-signer-tls-ca-file=/etc/qrysm/signer-ca.pem \
  --beacon-rpc-provider=127.0.0.1:4000 \
  --voluntary-exit-public-keys=0x<public key>
```

The exit is signed for the network the command is configured for, so run it with the validator client's network
configuration; the signer refuses a request for another network.

The signer signs `VOLUNTARY_EXIT` only when it runs with `--voluntary-exit-signing-enabled`. Otherwise it answers
403 `ARTIFACT_TYPE_DISABLED`, and the command stops at the first key, submits nothing and fails with `remote signer
does not sign voluntary exits (start it with --voluntary-exit-signing-enabled to sign them)`. An exit cannot be undone:
enable the option for the exit only, and restart the signer without it afterwards.

## Moving a validator from local keys to the remote signer

These are the validator client's steps. The signer's steps (loading the keys, importing the slashing protection
history, fencing its store with a high watermark, starting and verifying it) are in go-remotesigner's
[migration runbook](https://github.com/rgeraldes24/go-remotesigner/blob/m/v1.1-prep/docs/operations/migration.md),
and step 3 below is where they fit. The order matters: a key must never sign in two places, and the signer must hold a
key's history before it signs anything with that key.

1. **Stop the validator client**, and keep whatever restarts it (a service manager, a container restart policy) from
   starting it again with the local keys.
2. **Export the slashing protection history**, after the last signature:

   ```
   validator slashing-protection-history export \
     --datadir=<wallet-dir>/direct \
     --slashing-protection-export-dir=<dir>
   ```

   This writes `<dir>/slashing_protection.json` (EIP-3076). `<wallet-dir>/direct` is where a local wallet keeps
   `validator.db`; if the validator client ran with its own `--datadir`, export from that directory instead. Note the
   public keys to move (`validator accounts list`): they go into the key file of step 4.
3. **Move the keys and their history to the signer**, as the runbook describes: export the keys with
   `validator accounts backup` ([keystores from a qrysm
   wallet](https://github.com/rgeraldes24/go-remotesigner/blob/m/v1.1-prep/docs/operations/keystores.md#5-from-a-qrysm-wallet)),
   import `slashing_protection.json`, set the high watermark and start the signer on the same network. Once the signer
   has loaded the keys, remove them from the wallet (`validator accounts delete`), so that a validator client started
   on the wallet by mistake cannot sign with them.
4. **Start the validator client in remote mode** on the same database:

   ```
   validator \
     --datadir=<wallet-dir>/direct \
     --validators-external-signer-url=https://signer.internal:9000 \
     --validators-external-signer-key-file=/etc/qrysm/remote-keys.txt \
     --validators-external-signer-auth-token-file=/etc/qrysm/signer-auth-token \
     --validators-external-signer-tls-ca-file=/etc/qrysm/signer-ca.pem \
     <the other flags, as before>
   ```

   The key file lists the moved validators' public keys, one `0x` key per line. The token file holds the signer's
   sign-scope token (`<data-path>/auth-token` on the signer), never its admin token. Once the chain has started, the
   validator client checks the signer's capabilities against the chain and stops, naming each difference, if they do
   not match.
5. **Verify**, as the runbook's section 5 describes. While the signer's high watermark fences the store, the validator
   client logs each refused duty at WARN with the reason `SLASHING_ABOVE_HIGH_WATERMARK`; duties are signed again
   once the fence is lifted.

To move back, stop the validator client, export the history from the signer (runbook section 7), import it with
`validator slashing-protection-history import --datadir=<wallet-dir>/direct --slashing-protection-json-file=<file>`,
import the keys into the wallet, and start the validator client without the remote signer flags.

### The database directory

In remote mode the validator client opens no wallet, so the default location of its slashing protection database
changes: without `--datadir` it keeps `validator.db` in `--wallet-dir` itself, while a local wallet keeps it in
`<wallet-dir>/direct`. Pass `--datadir=<wallet-dir>/direct` (or the `--datadir` the local validator client used) so
that the validator client keeps the history it signed with its local keys. It checks every block and attestation
against that history before broadcasting it, a second gate behind the signer's.

Without `--datadir`, the validator client refuses to start in remote mode when `<wallet-dir>/direct/validator.db`
exists and the database it would use is missing or holds no signing history; the message names both paths and the
`--datadir` to set. An explicit `--datadir` is used as given: check the `databasePath` of the `Checking DB` line it
logs at startup.

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
