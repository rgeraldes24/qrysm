"""
SSZ proto templating rules.

These rules allow for variable substitution for hardcoded tag values like ssz-size and ssz-max.

"""

####### Configuration #######

mainnet = {
    "block_roots.size": "8192,32",  # SLOTS_PER_HISTORICAL_ROOT, [32]byte
    "state_roots.size": "8192,32",  # SLOTS_PER_HISTORICAL_ROOT, [32]byte
    "zond1_data_votes.size": "2048",  # SLOTS_PER_ZOND1_VOTING_PERIOD
    "randao_mixes.size": "65536,32",  # EPOCHS_PER_HISTORICAL_VECTOR, [32]byte
    "previous_epoch_attestations.max": "4096",  # MAX_ATTESTATIONS * SLOTS_PER_EPOCH
    "current_epoch_attestations.max": "4096",  # MAX_ATTESTATIONS * SLOTS_PER_EPOCH
    "slashings.size": "8192",  # EPOCHS_PER_SLASHINGS_VECTOR
    "sync_committee_bits.size": "16", #SYNC_COMMITTEE_SIZE
    "sync_committee_bytes.size": "2",
    "sync_committee_bits.type": "github.com/theQRL/go-bitfield.Bitvector16",
    "sync_committee_participation_bytes.size": "2",
    "sync_committee_participation_bits.type": "github.com/theQRL/go-bitfield.Bitvector16",
    "withdrawal.size": "16",
}

minimal = {
    "block_roots.size": "64,32",
    "state_roots.size": "64,32",
    "zond1_data_votes.size": "32",
    "randao_mixes.size": "64,32",
    "previous_epoch_attestations.max": "1024",
    "current_epoch_attestations.max": "1024",
    "slashings.size": "64",
    "sync_committee_bits.size": "32",
    "sync_committee_bytes.size": "4",
    "sync_committee_bits.type": "github.com/theQRL/go-bitfield.Bitvector32",
    "sync_committee_participation_bytes.size": "1",
    "sync_committee_aggregate_bits.type": "github.com/theQRL/go-bitfield.Bitvector8",
    "withdrawal.size": "4",
}

###### Rules definitions #######

def _ssz_proto_files_impl(ctx):
    """
    ssz_proto_files implementation performs expand_template based on the value of "config".
    """
    outputs = []
    if (ctx.attr.config.lower() == "mainnet"):
        subs = mainnet
    elif (ctx.attr.config.lower() == "minimal"):
        subs = minimal
    else:
        fail("%s is an unknown configuration" % ctx.attr.config)

    for src in ctx.attr.srcs:
        output = ctx.actions.declare_file(src.files.to_list()[0].basename)
        outputs.append(output)
        ctx.actions.expand_template(
            template = src.files.to_list()[0],
            output = output,
            substitutions = subs,
        )

    return [DefaultInfo(files = depset(outputs))]

ssz_proto_files = rule(
    implementation = _ssz_proto_files_impl,
    attrs = {
        "srcs": attr.label_list(mandatory = True, allow_files = [".proto"]),
        "config": attr.string(mandatory = True),
    },
)
