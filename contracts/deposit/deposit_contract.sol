// ┏━━━┓━┏┓━┏┓━━┏━━━┓━━┏━━━┓━━━━┏━━━┓━━━━━━━━━━━━━━━━━━━┏┓━━━━━┏━━━┓━━━━━━━━━┏┓━━━━━━━━━━━━━━┏┓━
// ┃┏━━┛┏┛┗┓┃┃━━┃┏━┓┃━━┃┏━┓┃━━━━┗┓┏┓┃━━━━━━━━━━━━━━━━━━┏┛┗┓━━━━┃┏━┓┃━━━━━━━━┏┛┗┓━━━━━━━━━━━━┏┛┗┓
// ┃┗━━┓┗┓┏┛┃┗━┓┗┛┏┛┃━━┃┃━┃┃━━━━━┃┃┃┃┏━━┓┏━━┓┏━━┓┏━━┓┏┓┗┓┏┛━━━━┃┃━┗┛┏━━┓┏━┓━┗┓┏┛┏━┓┏━━┓━┏━━┓┗┓┏┛
// ┃┏━━┛━┃┃━┃┏┓┃┏━┛┏┛━━┃┃━┃┃━━━━━┃┃┃┃┃┏┓┃┃┏┓┃┃┏┓┃┃━━┫┣┫━┃┃━━━━━┃┃━┏┓┃┏┓┃┃┏┓┓━┃┃━┃┏┛┗━┓┃━┃┏━┛━┃┃━
// ┃┗━━┓━┃┗┓┃┃┃┃┃┃┗━┓┏┓┃┗━┛┃━━━━┏┛┗┛┃┃┃━┫┃┗┛┃┃┗┛┃┣━━┃┃┃━┃┗┓━━━━┃┗━┛┃┃┗┛┃┃┃┃┃━┃┗┓┃┃━┃┗┛┗┓┃┗━┓━┃┗┓
// ┗━━━┛━┗━┛┗┛┗┛┗━━━┛┗┛┗━━━┛━━━━┗━━━┛┗━━┛┃┏━┛┗━━┛┗━━┛┗┛━┗━┛━━━━┗━━━┛┗━━┛┗┛┗┛━┗━┛┗┛━┗━━━┛┗━━┛━┗━┛
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┃┃━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┗┛━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// SPDX-License-Identifier: CC0-1.0

pragma solidity 0.6.11;

// This interface is designed to be compatible with the Vyper version.
/// @notice This is the Ethereum 2.0 deposit contract interface.
/// For more information see the Phase 0 specification under https://github.com/ethereum/eth2.0-specs
interface IDepositContract {
    /// @notice A processed deposit event.
    event DepositEvent(
        bytes pubkey,
        bytes withdrawal_credentials,
        bytes amount,
        bytes signature,
        bytes index
    );

    /// @notice Submit a Phase 0 DepositData object.
    /// @param pubkey A BLS12-381 public key.
    /// @param withdrawal_credentials Commitment to a public key for withdrawals.
    /// @param signature A BLS12-381 signature.
    /// @param deposit_data_root The SHA-256 hash of the SSZ-encoded DepositData object.
    /// Used as a protection against malformed input.
    function deposit(
        bytes calldata pubkey,
        bytes calldata withdrawal_credentials,
        bytes calldata signature,
        bytes32 deposit_data_root
    ) external payable;

    /// @notice Query the current deposit root hash.
    /// @return The deposit root hash.
    function get_deposit_root() external view returns (bytes32);

    /// @notice Query the current deposit count.
    /// @return The deposit count encoded as a little endian 64-bit number.
    function get_deposit_count() external view returns (bytes memory);
}

// Based on official specification in https://eips.ethereum.org/EIPS/eip-165
interface ERC165 {
    /// @notice Query if a contract implements an interface
    /// @param interfaceId The interface identifier, as specified in ERC-165
    /// @dev Interface identification is specified in ERC-165. This function
    ///  uses less than 30,000 gas.
    /// @return `true` if the contract implements `interfaceId` and
    ///  `interfaceId` is not 0xffffffff, `false` otherwise
    function supportsInterface(bytes4 interfaceId) external pure returns (bool);
}

// This is a rewrite of the Vyper Eth2.0 deposit contract in Solidity.
// It tries to stay as close as possible to the original source code.
/// @notice This is the Ethereum 2.0 deposit contract interface.
/// For more information see the Phase 0 specification under https://github.com/ethereum/eth2.0-specs
contract DepositContract is IDepositContract, ERC165 {
    uint constant DEPOSIT_CONTRACT_TREE_DEPTH = 32;
    // NOTE: this also ensures `deposit_count` will fit into 64-bits
    uint constant MAX_DEPOSIT_COUNT = 2**DEPOSIT_CONTRACT_TREE_DEPTH - 1;

    bytes32[DEPOSIT_CONTRACT_TREE_DEPTH] branch;
    uint256 deposit_count;

    bytes32[DEPOSIT_CONTRACT_TREE_DEPTH] zero_hashes;

    constructor() public {
        // Compute hashes in empty sparse Merkle tree
        for (uint height = 0; height < DEPOSIT_CONTRACT_TREE_DEPTH - 1; height++)
            zero_hashes[height + 1] = sha256(abi.encodePacked(zero_hashes[height], zero_hashes[height]));
    }

    function get_deposit_root() override external view returns (bytes32) {
        bytes32 node;
        uint size = deposit_count;
        for (uint height = 0; height < DEPOSIT_CONTRACT_TREE_DEPTH; height++) {
            if ((size & 1) == 1)
                node = sha256(abi.encodePacked(branch[height], node));
            else
                node = sha256(abi.encodePacked(node, zero_hashes[height]));
            size /= 2;
        }
        return sha256(abi.encodePacked(
                node,
                to_little_endian_64(uint64(deposit_count)),
                bytes24(0)
            ));
    }

    function get_deposit_count() override external view returns (bytes memory) {
        return to_little_endian_64(uint64(deposit_count));
    }

    function deposit(
        bytes calldata pubkey,
        bytes calldata withdrawal_credentials,
        bytes calldata signature,
        bytes32 deposit_data_root
    ) override external payable {
        // Extended ABI length checks since dynamic types are used.
        require(pubkey.length == 2592, "DepositContract: invalid pubkey length");
        require(withdrawal_credentials.length == 32, "DepositContract: invalid withdrawal_credentials length");
        require(signature.length == 4595, "DepositContract: invalid signature length");

        // Check deposit amount
        require(msg.value >= 1 ether, "DepositContract: deposit value too low");
        require(msg.value % 1 gwei == 0, "DepositContract: deposit value not multiple of gwei");
        uint deposit_amount = msg.value / 1 gwei;
        require(deposit_amount <= type(uint64).max, "DepositContract: deposit value too high");

        // Emit `DepositEvent` log
        bytes memory amount = to_little_endian_64(uint64(deposit_amount));
        emit DepositEvent(
            pubkey,
            withdrawal_credentials,
            amount,
            signature,
            to_little_endian_64(uint64(deposit_count))
        );

        // Compute deposit data root (`DepositData` hash tree root)
        bytes32 pubkey_root = to_pubkey_root(pubkey);
        
        // CompilerError: Stack too deep, try removing local variables.
        bytes32 signature_root_1 = to_signature_root_1(signature);
        bytes32 signature_root_2 = to_signature_root_2(signature);
        bytes32 signature_root_3 = to_signature_root_3(signature);
    
        bytes32 signature_root = sha256(abi.encodePacked(
                sha256(abi.encodePacked(signature_root_1, signature_root_2)),
                signature_root_3
            ));


        bytes32 node = sha256(abi.encodePacked(
                sha256(abi.encodePacked(pubkey_root, withdrawal_credentials)),
                sha256(abi.encodePacked(amount, bytes24(0), signature_root))
            ));

        // Verify computed and expected deposit data roots match
        require(node == deposit_data_root, "DepositContract: reconstructed DepositData does not match supplied deposit_data_root");

        // Avoid overflowing the Merkle tree (and prevent edge case in computing `branch`)
        require(deposit_count < MAX_DEPOSIT_COUNT, "DepositContract: merkle tree full");

        // Add deposit data root to Merkle tree (update a single `branch` node)
        deposit_count += 1;
        uint size = deposit_count;
        for (uint height = 0; height < DEPOSIT_CONTRACT_TREE_DEPTH; height++) {
            if ((size & 1) == 1) {
                branch[height] = node;
                return;
            }
            node = sha256(abi.encodePacked(branch[height], node));
            size /= 2;
        }
        // As the loop should always end prematurely with the `return` statement,
        // this code should be unreachable. We assert `false` just to be safe.
        assert(false);
    }

    function supportsInterface(bytes4 interfaceId) override external pure returns (bool) {
        return interfaceId == type(ERC165).interfaceId || interfaceId == type(IDepositContract).interfaceId;
    }

    function to_little_endian_64(uint64 value) internal pure returns (bytes memory ret) {
        ret = new bytes(8);
        bytes8 bytesValue = bytes8(value);
        // Byteswapping during copying to bytes.
        ret[0] = bytesValue[7];
        ret[1] = bytesValue[6];
        ret[2] = bytesValue[5];
        ret[3] = bytesValue[4];
        ret[4] = bytesValue[3];
        ret[5] = bytesValue[2];
        ret[6] = bytesValue[1];
        ret[7] = bytesValue[0];
    }

    function to_pubkey_root(bytes calldata pubkey) internal pure returns (bytes32) {
        return sha256(abi.encodePacked(
                sha256(abi.encodePacked(
                    sha256(abi.encodePacked(
                        sha256(abi.encodePacked(
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[:64])),
                                    sha256(abi.encodePacked(pubkey[64:128]))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[128:192])),
                                    sha256(abi.encodePacked(pubkey[192:256]))
                                ))
                            )),
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[256:320])),
                                    sha256(abi.encodePacked(pubkey[320:384]))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[384:448])),
                                    sha256(abi.encodePacked(pubkey[448:512]))
                                ))
                            ))
                        )),
                        sha256(abi.encodePacked(
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[512:576])),
                                    sha256(abi.encodePacked(pubkey[576:640]))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[640:704])),
                                    sha256(abi.encodePacked(pubkey[704:768]))
                                ))
                            )),
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[768:832])),
                                    sha256(abi.encodePacked(pubkey[832:896]))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[896:960])),
                                    sha256(abi.encodePacked(pubkey[960:1024]))
                                ))
                            ))
                        ))
                    )),
                    sha256(abi.encodePacked(
                        sha256(abi.encodePacked(
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[1024:1088])),
                                    sha256(abi.encodePacked(pubkey[1088:1152]))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[1152:1216])),
                                    sha256(abi.encodePacked(pubkey[1216:1280]))
                                ))
                            )),
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[1280:1344])),
                                    sha256(abi.encodePacked(pubkey[1344:1408]))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[1408:1472])),
                                    sha256(abi.encodePacked(pubkey[1472:1536]))
                                ))
                            ))
                        )),
                        sha256(abi.encodePacked(
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[1536:1600])),
                                    sha256(abi.encodePacked(pubkey[1600:1664]))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[1664:1728])),
                                    sha256(abi.encodePacked(pubkey[1728:1792]))
                                ))
                            )),
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[1792:1856])),
                                    sha256(abi.encodePacked(pubkey[1856:1920]))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[1920:1984])),
                                    sha256(abi.encodePacked(pubkey[1984:2048]))
                                ))
                            ))
                        ))
                    ))
                )),
                sha256(abi.encodePacked(
                    sha256(abi.encodePacked(
                        sha256(abi.encodePacked(
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[2048:2112])),
                                    sha256(abi.encodePacked(pubkey[2112:2176]))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[2176:2240])),
                                    sha256(abi.encodePacked(pubkey[2240:2304]))
                                ))
                            )),
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[2304:2368])),
                                    sha256(abi.encodePacked(pubkey[2368:2432]))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(pubkey[2432:2496])),
                                    sha256(abi.encodePacked(pubkey[2496:2560]))
                                ))
                            ))
                        )),
                        sha256(abi.encodePacked(pubkey[2560:], bytes32(0)))
                    )),
                    bytes32(0)
                ))                            
            ));
    }

    function to_signature_root_1(bytes calldata signature) internal pure returns (bytes32) {
        return sha256(abi.encodePacked(
                        sha256(abi.encodePacked(
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[:64])),
                                        sha256(abi.encodePacked(signature[64:128]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[128:192])),
                                        sha256(abi.encodePacked(signature[192:256]))
                                    ))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[256:320])),
                                        sha256(abi.encodePacked(signature[320:384]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[384:448])),
                                        sha256(abi.encodePacked(signature[448:512]))
                                    ))
                                ))
                            )),
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[512:576])),
                                        sha256(abi.encodePacked(signature[576:640]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[640:704])),
                                        sha256(abi.encodePacked(signature[704:768]))
                                    ))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[768:832])),
                                        sha256(abi.encodePacked(signature[832:896]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[896:960])),
                                        sha256(abi.encodePacked(signature[960:1024]))
                                    ))
                                ))
                            ))
                        )),
                        sha256(abi.encodePacked(
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[1024:1088])),
                                        sha256(abi.encodePacked(signature[1088:1152]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[1152:1216])),
                                        sha256(abi.encodePacked(signature[1216:1280]))
                                    ))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[1280:1344])),
                                        sha256(abi.encodePacked(signature[1344:1408]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[1408:1472])),
                                        sha256(abi.encodePacked(signature[1472:1536]))
                                    ))
                                ))
                            )),
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[1536:1600])),
                                        sha256(abi.encodePacked(signature[1600:1664]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[1664:1728])),
                                        sha256(abi.encodePacked(signature[1728:1792]))
                                    ))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[1792:1856])),
                                        sha256(abi.encodePacked(signature[1856:1920]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[1920:1984])),
                                        sha256(abi.encodePacked(signature[1984:2048]))
                                    ))
                                ))
                            ))
                        ))
                    ));
    } 

    function to_signature_root_2(bytes calldata signature) internal pure returns (bytes32) {
        return sha256(abi.encodePacked(
                        sha256(abi.encodePacked(
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[2048:2112])),
                                        sha256(abi.encodePacked(signature[2112:2176]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[2176:2240])),
                                        sha256(abi.encodePacked(signature[2240:2304]))
                                    ))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[2304:2368])),
                                        sha256(abi.encodePacked(signature[2368:2432]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[2432:2496])),
                                        sha256(abi.encodePacked(signature[2496:2560]))
                                    ))
                                ))
                            )),
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[2560:2624])),
                                        sha256(abi.encodePacked(signature[2624:2688]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[2688:2752])),
                                        sha256(abi.encodePacked(signature[2752:2816]))
                                    ))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[2816:2880])),
                                        sha256(abi.encodePacked(signature[2880:2944]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[2944:3008])),
                                        sha256(abi.encodePacked(signature[3008:3072]))
                                    ))
                                ))
                            ))
                        )),
                        sha256(abi.encodePacked(
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[3072:3136])),
                                        sha256(abi.encodePacked(signature[3136:3200]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[3200:3264])),
                                        sha256(abi.encodePacked(signature[3264:3328]))
                                    ))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[3328:3392])),
                                        sha256(abi.encodePacked(signature[3392:3456]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[3456:3520])),
                                        sha256(abi.encodePacked(signature[3520:3584]))
                                    ))
                                ))
                            )),
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[3584:3648])),
                                        sha256(abi.encodePacked(signature[3648:3712]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[3712:3776])),
                                        sha256(abi.encodePacked(signature[3776:3840]))
                                    ))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[3840:3904])),
                                        sha256(abi.encodePacked(signature[3904:3968]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[3968:4032])),
                                        sha256(abi.encodePacked(signature[4032:4096]))
                                    ))
                                ))
                            ))
                        ))
                    ));
    }

    function to_signature_root_3(bytes calldata signature) internal pure returns (bytes32) {
        bytes memory arr = new bytes(13);
        arr[0] = 0;
        arr[1] = 0;
        arr[2] = 0;
        arr[3] = 0;
        arr[4] = 0;
        arr[5] = 0;
        arr[6] = 0;
        arr[7] = 0;
        arr[8] = 0;
        arr[9] = 0;
        arr[10] = 0;
        arr[11] = 0;
        arr[12] = 0;

        return sha256(abi.encodePacked(
                    sha256(abi.encodePacked(
                        sha256(abi.encodePacked(
                            sha256(abi.encodePacked(
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[4096:4160])),
                                        sha256(abi.encodePacked(signature[4160:4224]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[4224:4288])),
                                        sha256(abi.encodePacked(signature[4288:4352]))
                                    ))
                                )),
                                sha256(abi.encodePacked(
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[4352:4416])),
                                        sha256(abi.encodePacked(signature[4416:4480]))
                                    )),
                                    sha256(abi.encodePacked(
                                        sha256(abi.encodePacked(signature[4480:4544])),
                                        sha256(abi.encodePacked(signature[4544:], arr))
                                    ))
                                ))
                            )),
                            bytes32(0)
                        )),
                        bytes32(0)
                    )),
                    bytes32(0)
                ));
    }


    
}