package blocks_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/signing"
	state_native "github.com/theQRL/qrysm/v4/beacon-chain/state/state-native"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation"
	"github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation/aggregation"
	attaggregation "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation/aggregation/attestations"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func TestProcessAggregatedAttestation_OverlappingBits(t *testing.T) {
	beaconState, privKeys := util.DeterministicGenesisState(t, 100)
	data := util.HydrateAttestationData(&zondpb.AttestationData{
		Source: &zondpb.Checkpoint{Epoch: 0, Root: bytesutil.PadTo([]byte("hello-world"), 32)},
		Target: &zondpb.Checkpoint{Epoch: 0, Root: bytesutil.PadTo([]byte("hello-world"), 32)},
	})
	participationBits1 := bitfield.NewBitlist(3)
	participationBits1.SetBitAt(0, true)
	participationBits1.SetBitAt(1, true)
	att1 := &zondpb.Attestation{
		Data:              data,
		ParticipationBits: participationBits1,
	}

	cfc := beaconState.CurrentJustifiedCheckpoint()
	cfc.Root = bytesutil.PadTo([]byte("hello-world"), 32)
	require.NoError(t, beaconState.SetCurrentJustifiedCheckpoint(cfc))
	//require.NoError(t, beaconState.AppendCurrentEpochAttestations(&zondpb.PendingAttestation{}))

	committee, err := helpers.BeaconCommitteeFromState(context.Background(), beaconState, att1.Data.Slot, att1.Data.CommitteeIndex)
	require.NoError(t, err)
	attestingIndices1, err := attestation.AttestingIndices(att1.ParticipationBits, committee)
	require.NoError(t, err)
	sigs := make([][]byte, len(attestingIndices1))
	for i, indice := range attestingIndices1 {
		sb, err := signing.ComputeDomainAndSign(beaconState, 0, att1.Data, params.BeaconConfig().DomainBeaconAttester, privKeys[indice])
		require.NoError(t, err)
		sigs[i] = sb
	}
	att1.Signatures = sigs

	participationBits2 := bitfield.NewBitlist(3)
	participationBits2.SetBitAt(1, true)
	participationBits2.SetBitAt(2, true)
	att2 := &zondpb.Attestation{
		Data:              data,
		ParticipationBits: participationBits2,
	}

	committee, err = helpers.BeaconCommitteeFromState(context.Background(), beaconState, att2.Data.Slot, att2.Data.CommitteeIndex)
	require.NoError(t, err)
	attestingIndices2, err := attestation.AttestingIndices(att2.ParticipationBits, committee)
	require.NoError(t, err)
	sigs = make([][]byte, len(attestingIndices2))
	for i, indice := range attestingIndices2 {
		sb, err := signing.ComputeDomainAndSign(beaconState, 0, att2.Data, params.BeaconConfig().DomainBeaconAttester, privKeys[indice])
		require.NoError(t, err)
		sigs[i] = sb
	}
	att2.Signatures = sigs

	_, err = attaggregation.AggregatePair(att1, att2)
	assert.ErrorContains(t, aggregation.ErrBitsOverlap.Error(), err)
}

func TestVerifyAttestationNoVerifySignature_IncorrectSlotTargetEpoch(t *testing.T) {
	beaconState, _ := util.DeterministicGenesisState(t, 1)

	att := util.HydrateAttestation(&zondpb.Attestation{
		Data: &zondpb.AttestationData{
			Slot:   params.BeaconConfig().SlotsPerEpoch,
			Target: &zondpb.Checkpoint{Root: make([]byte, 32)},
		},
	})
	wanted := "slot 32 does not match target epoch 0"
	err := blocks.VerifyAttestationNoVerifySignatures(context.TODO(), beaconState, att)
	assert.ErrorContains(t, wanted, err)
}

func TestVerifyAttestationNoVerifySignature_BadAttIdx(t *testing.T) {
	beaconState, _ := util.DeterministicGenesisState(t, 100)
	participationBits := bitfield.NewBitlist(3)
	participationBits.SetBitAt(1, true)
	var mockRoot [32]byte
	copy(mockRoot[:], "hello-world")
	att := &zondpb.Attestation{
		Data: &zondpb.AttestationData{
			CommitteeIndex: 100,
			Source:         &zondpb.Checkpoint{Epoch: 0, Root: mockRoot[:]},
			Target:         &zondpb.Checkpoint{Epoch: 0, Root: make([]byte, 32)},
		},
		ParticipationBits: participationBits,
	}
	var zeroSig [dilithium.CryptoBytes]byte
	att.Signatures = [][]byte{zeroSig[:]}
	require.NoError(t, beaconState.SetSlot(beaconState.Slot()+params.BeaconConfig().MinAttestationInclusionDelay))
	ckp := beaconState.CurrentJustifiedCheckpoint()
	copy(ckp.Root, "hello-world")
	require.NoError(t, beaconState.SetCurrentJustifiedCheckpoint(ckp))
	//require.NoError(t, beaconState.AppendCurrentEpochAttestations(&zondpb.PendingAttestation{}))
	err := blocks.VerifyAttestationNoVerifySignatures(context.TODO(), beaconState, att)
	require.ErrorContains(t, "committee index 100 >= committee count 1", err)
}

func TestConvertToIndexed_OK(t *testing.T) {
	sig47, err := hex.DecodeString("8e654c3ed5da44d83a1b6c83bdfd601fb73dfb3e2247d29ad068c7e6c25274d4b543d127db4e03b89d9c4795ae8b531859e1c4caa9ef61a84f0b31cb8d396dbfdcbbae050c5733189fcbc0d02f2500ebb12ad611bd87eba50078334d2d2ac1ce074b8e0aacc6b1a94a379c1bd231239e757aedb07dd5cb711c47236d959de596c1a164b3123a923930f3c78d1ca24b641eed8171db401078ddc6e6323f481340540700cf7942e1cd5532570e37327d6a4be1c6022d0eb4906a94a57f0400b7bbb10d1ed58cb6d418be6165b97075846171fabb02a64d59b9b8a942a132cdc1764bf8cd698c60dbf7d43078ba2591b6067ebd0b047d8ead8e233628f73388a3d20490394db580644b284c875d45b0ae425c82fd7fbb82a28b3e574123121683615d82b2de191892519999d3ab5c651ec588fe748e28cc4de9b1bcf716a5ecdc628b00e5a4ae0ab4086004af57a5aa6479ccf49f854688560e3f5dabe42f8a04b6bd044ff75c49827cdac25e4b92580d996f3a6c79586853f63dbab114732488ea40e3fd46797284623f36f1a3de0c5c8d4c985428e94f44f1665135e669270d9a99f86f085efc43b427ff3a31411b418ce067552e4df0f9ba9257b2f34c77f6677d44c353735a54007195ce2f6d42f7b4df027367449be253e457629386ca25c251aadde1d89f6714368ae8ea03aa0f655dab00f8b9720d39cea8814772d9a1cad90af2b5c444bb3146f3651718d22d87da236c97f6cbe947e009be29a012faf52f0e692bd062be7f89963f62b095821787b273e1c2e287d56b06ab69c757c181f909fb231219fb0f773ad07927a30d228893bedef5540a022dd8f9d533720b9731fd0eb66bcaa59ab28588ce72c6d868202f70d44d7a55c5ed02b7827e2b313ab237b6bb2564ee9626944ae8121e91376fe89e4fd0a50da6f181c587c694924984af67c9953aa4dc542eec83ac330953d2feadecd718551b1e10b4d40fbfb72e2e9b0e8dec856b5458b383bf090efd5b4502b7ebdde2588f42bb99c85b6ed62c28eb459d8a4f77abee4ad0ac4a676aa16e9f48331c7a5fc9e6d763f68a22bf5a7f7d0cabf62a1bd0f5f78bcacc5b30984e3975ed3b266b65c90c8dcf2074c265a2a2f19f68562e93f3ba1fbc7bcac6275b217054c6cd8c10506a488da7ce06a21800eb5313e74da5a4eb61ff1741bea9dd5cfa40ad0c1023126ab0c5a87afcfb725eff8a891c2e5d305b9a70bdd32c089f8aee40549f1c1345d8bc033b2680accb316cadd5d0a94b98168c94ec94963455f525236f7beb8bc26b7134de6b6bb69fcbdec97c9419100174324d594396e8fb3a9c711a90c5bf5170b99c515a9a7fce67d8671b3ed2ecea8abceec637ce572a4216a45c2ff7ca84e7f02e59a7a2a47cca089819995d09cebbd5a8cf5c632fe161f69a7cde35ab0ebc0369cb380274c34a067051af5b0f7d86b8ed380fc9577704db8293715cd088baeb8f59a158947997446552031db6b2e51d0806ee66a4fe783be9f9480ee1d918096d156656528dc015ebed0548ce2a0b1cd7af70d9dcda9c9052a565571d347f49c68b8405d63f51eb668054e50ece1a35c506e9675e0d405718e42022b53a2958c553f59c54f58e28dacf62104f2f1f7865c78b43e87d632b2e6edf26db0c79bffca1f26d9e1a4025820cfce74e6218ad7c19f39453bf4a6fc574ec3688972b47a25e066cd9b04b10295c000e3b40943c062ca19aea599ae158ab0f694b3f84838062ca5673cdfb66e07fed87193303e2cff1d5eb6b2844369f8770e863ae1312843d021076e08b29c1647d96899cb53ee97e3b646d6049df38e4f1fc7958aebdfcb51940025e9454a8c69065f8d65c2b138d31a3de25480d9e5530e73fbe138451b0a637e069d1bb2be89647d326f0884f2eed195bff3c7ee498c92bf94e9fb353cabce52c45195f854ff9ddf8eb2de99d2c22f95889379741a6142e1d9b751b1fc6ca5b136ab4a7de56397bce40002f2c124e3778f09a2ebb1b0f80cc0d3e040239d2fb5506c23f62003fc1b96f3977fec12242c234a73053f468f8025310ce8bc597f6ca1f9fedc305eae92b03e66ba0f87666582ceb709001f0d3916544d812d44f986b87416cca7ce66f00344260bbc507adf9ec6a97db6ac9773d50cdf37491d282610eae3b78fd63a4428a796e34bc25a97a8ef874624ae5dd16b2dd9d147564441eddbb6bce012eccb81fa7f06f13cf5e0583ce823c82c98c0cee92066603e37798e9a042727d1371ae61e31db0df4736865136d39e4775290411a973192f3fab7aa0f1f3170bbcefe0e3d6f26b0a032d6bc4143d4d221b28589d4e46177e2d62b6769e6caac027c1f6d3d5124975200f7556a9471f2f53817b5b9414e6acdbe3ffbbc0d75856a33f6a64dd40c91750df8d9fa86cedcd3c926ce3226defce947d36445f24d2ef83ecef04ac3adc25347d0fce0c1490e732a1f45527023e8e5d009bba5b908e533991eaeb437071a38ceb1157853eaa636e918283240693fe9e961e93eb64606341e200c3e65eed1b47316ce62e2f7b020857386d2733f973f6a390a8f5614c6f6ba8702715459bb8cc19cd386f135143bd6cf64a7bf738a22816b13709c5689e0f11d84562b32ffe7a9931aad184851146e047271913589e320921c88a2d6340797303fed6593854ff890357e831cdc3dcb7b6425af54b7eec53cb51c18f9b1b3f12aed18d16ac2518a21607d81bb3eb3873727ad0796dd6c749477b4cd4e4b7d02f96bd3b5b11a9a3fd3f60914f2d9456c460d8865e068e3489bc91125a66cfaeea7d11f6dd78565cc95c1abccb0ec96425a806266188fd63b4dd0e1a3e042f27d71fb96921354ce910bfe0b07a0fd4067fff0ab41cfda0be8b8430147835b522395d301df0750e5cda2d0e82fb51e28007fde876100f45f3e3b86e06c6ae94ead51c093c8b2ece5023ad3eacb467e6e763239aa30f226335d554b9fe423e051eff7c885668e632fb2452c03e657a9f5a142fe44b8c0a739af26e2e5f675408c120ea5ae6753c62ace06250a9478b48f478904f30053ba1c611902d1a37a844ae90d64e5f56c32421e4c9b5976f66cbdb7037530a7b94e9f252ba2a7e75740b3f741866ee14bb75157a00d4fbe732222933d400d786ee4ef4abf780d6e2955c9decafd4c6c2960b0996749021d8d093a0437b4dd92e8fb9fb412bb0de32c096a42fd3dcca97ed5158e05b6c6b237d985b16740a98a409ba4938f7f6e26e57f633770ad134ca076e9483358cad88d0a791eda0efc26f25edd25c4342c07408846f7b29de719955ae873a39b7b6788bd67b3acf568c19fe18abcc370cf919e5f0c098ebb0aebf49ea3872763f82443342dfd147204e115607fd0bb02f8c8b9f729010ab7939847f177be6baf9c6f6f67f8f346f925b86daac4490d594ede861c5a19795c5995193ea84ed5d2fbbbb91af4b885285fd418ff9ed75272c24ea815cf10f7a8217a0ef9a9b3b2b48f8f7987851e9872f19358610cb967c6f1c4e02b59e1f1197e93ff614b6dcd6a13b5f486efbbfde3356687def1b17168d9ebcf71ddc05a59216a749269f1f880cb0267e3d2241c156b52cf6da55b8a6ed205e7b0d6aad12f8354d403ff99dc74aa81b58296d3cb5e94fb853e3421c19237475fec05df80a958863c2bc30576b1ec9ea8ff304ebfea52c8d579ea68fa60c02cd1c4620ab1fff92e6803dc1c5bdaa0f4470df11f630a0f7907d52d91371f0c46b7d338096110df7ec5e1af6133743808a1a5c6bf6cc875871586edd18b907bffd34e27689d34f74db47bef849cd192119db130b49e4cf4cb96d2ad85464d5432efd8b56e8001ea0225b8fd4d02b03d0b4651880bc47d9cb7519c00f6dd2b1a8a09c5b675732d8a165af045689bc0812c61d221a3f01a72388a0abd4d59a90b553bb39668bb2b7bd1d616e1fb97ae15ae609b2b73dcb8641fce461f4d23cb32b868265e2b9c1f8b9be664c6ec4c3d591dfdd9b16b37b03b1f643e81c696aabfb4de272d198b5c7ed64be63a3c63162c838f29ef235b37e5afe927d3c1956a0011d5919873e63c3fb2d470042a4c9d28c5b9c42165c2e837a0c9a6e34c5225834711fa32707382c813fcb52f044739d07ae8a1dafa6fec4f5e41eab45dead869ba4f51df82f843fc35e38269faac27689fe6f12587ca396a24696074ed2dff8b15c773a46500fa2c743a66a97f5aaf05e2d6ac4f84a3249484eb28c94e877c3a3eb7a2d1b667ebd48477bfbbee6cdaa48017b751b74e130d07fd8c8fe7811bc5a533f5ff7ac7813ae803b998b2e862673852d7b779542e5cfdb57be449fc380b9ef31f545a0de5fa74bd407e5ebc1c436f919457ef6e028767eb3091563b02ab08f4d4d27398e35d658af5d27d0ac127eded2c4f63bc722c97147771c40ced3846c7cb8813305e45d1cc50264b54441f97c9799e871d17759fedc3a64d64b5a25e74b8e873f5bc6175f2743602205ba0feb2e522462d68edaf3af818229dfe4021be92e08d27746d4de515bcdb7842d766d9fb9088bca153e5070a81d36f2decb52592a82f610d6dd48758605337bf190ed1e396dcb20f78cde6917642d30996cc5fd15efb5d4cd5b0dd842b6d0e7bf674b3a1425edd5b0e63819ae19ddbc381343ae27a0574ab3a00d5c84ff69386679abf443bd4ab757c1c20248ce3671de30873e8e2cdd09bf350acfd097f89494aaa0b552807ffb307987090e17adb19b342cbfed2c3bdd6062724b4c687de6a4e7d18d9e430f5e49bdf59f9abddb54fbc27e33c7f3a229af33cfe94cf91b4628f796b86307a1f431977feb0b5daf1b6486eb67542f14401c6a5fe40fbb7e0890ea17b6ddc7f006022a60c02d654a7118e1cb2eaf8e69f4d2a0fd0932e8715ec59f6b8c1b16f9d19df76e083ea0a1a801d26e6b3577134e2bbc4c5fb8ebcf575cf5a04d0b3895926b376759108f4a254da3ccba3ca6b2388439010b966b0d0c2438c3631f270d165f012fc4ee4cd021feb9df6a73340e0271bf1200d734f6b8833f8145dfbad06b3f6f93e965bd536880359efb5eae2360d21f0f9dc18fb9afe497927d96f6dc710c07ccdb5fc832ec2874a6063d75c4feb211ac405b62f77763c6df5bc330ae6d05bf23f0024ed693cb126d96bf4f2dff6e73f6a42ded17fcd19a65420d13bc73ab93d1bfe5c2aca0dbc9d16e2e003c7391bcbe931274898f70a7cf8732237218ff0e6b45040049f7344aa5b60353adbc2aa292726d2378f57348e5873eb0737200da552b1b5ec0994dcaa0fc200e7dad909fd76a2a113690ad1bae96b5c70f7807419361a3749585125a889f78627acc211d291f561706ff90d7bafc08b02a8292cd06f5eb8fbf92ce5ba5fef2d58eaeb53ccfc5aa11ad947fbf44904caea4c071e4ab28f55f4c7fc224b44f650b21391e5c01acb208cfc266f487a5578fe968077dbfdfafefde41ed71aadaf3538a2566d6828ef455af45500101f3b1ebbe66a9174c4c0757e5cbb36adb3e1b8922469e71eb5bf50802271c44f8b0062c5fddd2469d93c2b225d98b3bea447d39187bce743b0078bb810b2552d94fae0e6a0c85988f62087e98955943e7137f0f7ffa3cade593d0e38e0733c62fa1e0c90c6437c5da00b6517f33aec1b0bf684be9893960ff0caff2373b2869ab9dcf411bbf05daf5e55277414cbe8a69a251e99d0f7ba47ccc63d9045b59c5671009fd516c33dd748b14dc9e536283d97d8ac35a3b84b3330ba0556e52872271e7482b4ae22f8264e31db17d99b39334ac89e2f60cf66da6f1ec9caf1175254a448555e667c57c28f559800045660b4cda0f3b7b566c244995df4a30014ae04be2839a295de6564c19c746c13fa4fcbd27f19b9ef5a8538d3c1135284f12c3a82a4cd1671aae7d5571a2522b3f7b993618361a462028caa34b69885b58747ad181baa257fed7d77e35e1fc10c1a36dc1cc715808d97d9cec185741f90ee52ec7c742331f04e5ee47d9e4292a53c885e0287163f156781480f8a36a7885cb09543f847030031436ebc285472e3b390ea62466f696b4e328c62e2150fe629fe7cb7027d54e56b2a0b7d0bf9dd0b80d238a03cd9972cf1334d0ae3422f4095a138dd06f4957cdf0404b811a040cb48164ae916639971074a46eae9a83c88be42b3573b3d668609e74c6036ff6250b3fe0767f05695c59a18c0cde1411ab3197a6ad79c456bcdffa027a8f39a0ac1f9b24bd24f034af47db77c110c4aceb791789923994baec303d1c8d5cbbc315935c5f3ce9ee076fb7ebcbea2195803a6c5298151d75351f5e44042e295911e9a7a4365d504a45480e7c283668e038ae41916625db3ce14fbc4ee33ed2d915232c060e191c364575a1b6c1ec03051e4445517b94b0ce393e4d788eaef6f9ff0d1a3f5f699afe041432373849729bc6ccf1f60e0f19293c464bdee53d4c586f7b8e8f042f395388979fcbf1000b151e25313a414a")
	require.NoError(t, err)
	sig43, err := hex.DecodeString("acb58857ed5b045e639fe93633e389420a73cefd543ba79e5e94b922fab10ee353508e47e7f11191b1d8aed267914f7b4992b7ea862071e8de468233b31a36e6e13dca841ee4a80df1826fb0c0d3d3b98557d41e2069cf692a254830c38574b1b55d9d442cbd513ace3a8da986c9031403ec93e886c7a735e3721acbe957d5b5b712e921a075852aa6576b1b2a40bb4ebd100fa72b59ed46e28987398eb713392c7fc086ad5c4cc0e717a471b0cf530049eae94f142c13d87a5f409505d1587c267f43215ed647a0a26f33589a6508064f7885fe360edb2e42684fc45547ba7b66fbe023714795a2f9fefbfce9fe858d0c86e1823623f003acae3473aaca518cd4a67b2b1d377c68868566ae7f1e382af21bce09a1921ab2f37fbf79b349e30e2fccfbdc5554a98ee0e0b61171f300b85221c16040c9e5c35e3498dd1c96f011f39eefdbfbfdd4465f46475687106cdb980cbf27e899bbcf1b08eaae75867e7a15ed3a5267dcbfba05d5ef30a3db4e84ab0846b3c57e2c7048597e4bf5d046cba707c52dd688b9546e795b18fca4f541c589637ffeb9acfb9f715ef26621e236021a18ad9b3324b2c2c94678bbd2de2c870dd389cd6f08a3698a2c11486449526a57f6a39d8def3eb9de3a151e4f719dc0f3af20ca405c5009431dae2c6f5c9332e0373aa2258ccdafc1fe44683f55939f4c97e116585890b7b63889e641495a4d0f05cb7500c9c3aac3df68f84257384ce1e7777d35fc85113a8f62f439121236ff435e1cc6b3cf3c9c0e29e7a1b8442ddd41f2dc6d3d3ce9c1a0606cb33e2c9e1c02ea4ef822fc994d7d3e691b76223c3d8b4e67c8b29f144a40b58e470b8695f36932853f95b50b04ea76010754670aee472e1fc6b98102f7ecf35efc27a26e4894a07b094b1613d10b9acacadcf02e635ca3ff67dddd4b893082e53e7553f3d91e25c5c60bf1658c72a116fcdec7ba754cb561beaf5dcd3157341e623ec3ae4c3e2dd242264a7c1aa79c7911dfe769054644a24b58a4e2d738e294f20fbfa99e634ab1cb99421c8b708fbb67161703e919f8e0a9b0ecb667c7a56c8140ee19960c6f55d8e40a0002b1dbf347fccb802fb2f6b47c477a1f88b198de545011462143a79a11dd9af41954c54c5ba5517c33cdbef8f2d1f4ea8dd3ccfe2954364ee72e33f3e8778fc7230354d37b2c61f9ce31feb1eb9e51dbd5d05283fac8dbba98cdfd0f5ed7de138ef79a9d947d6a146786f1f7904ec12054fc5335ec6a879568107dc090aee6353d1e4a53b74270d2657f7c4c5fc73efeb229c3cdd2c237cdc6a79f6246b5a709d0d6b08d2aa0683a5944a94e1e122f8562a4908df1ad8db7f5c5fbb42caf2b9f1961c586556e29f44c7313bfe2c5fcb4a34b1e1108802dfa421d4af3b8195d62184d1dd5112e430c0d61f2684974cea3e690d23465bc8a5b1744fbc12e57c3479928ca809df89690d8d2aea9ca896c82be6e1546aa431de20b26b1ebc7724f312e5cd3311ba6ea3daea562244ea00ddd915f4c41a43e1380f0765c0f9560e2ba9df24834181ff46bd1f2b9613f390aedc8161ab8dc26b813de95680af6911c53ee9397e9ca2cfc2aff7b5c15c7699f87ded57cc168c70a015af295744ba90934a2f14fa40ccfbd0bcd0eed23d75e9a83649b35cddf5bcf7a2d7e40439c142b41010cb821872c148c28cca6b828d232db095edb626f4030872a746db2a442102c816db2655cbd517c340d9ad6bad3b84b9f5c43d2959eb4052cb648936948f4d310cebfb8602dbf814e42937154a9e13b14d1fa75940bb6e4786e0f46f8ab62dcb039f1fd0ad45fe10a19d99b9c713af519a5f89f1a7668ef97180d08d28e46b3a0d2bfe810ff93f5d41bb9bcf61a0ad889a9d3685bbddf4e9f2ee49832466d10f143f1e3b8429063462e89b7eb6e405568711632db8bbaa1e5aa7bfe2cc3ffd87b0893234c859b8e65a4968f889b365b29107f47ac9c8b8198cd4a80795e2f83526f3442910730711791af2a445d70de4f1f690c9371b92afd6425102cfcc05af5ed557d3451963958e7f709404bb418fd97fccbcb242234fedb23fb3ebfd455a5f7e96cf0ef417e596355191626f19ad4367d71c0b84f7dbd11290413ed48c15f22a1441aaf8fac3a0ee6e25bd6f976b59b87e4bd0e710a3a265094291c012c422a0048e2b00ed2b36d98d332fc77f20c0f71c609c601681d4cabf4e54083a31b5366c2e2866186843aa222ac7970c16ddec4402f9c5a6290a51c81185eceaf94a2678221f814f8f698d3528f7b7e2ce026c43a4c91114e0b5d8d6bd022736df72b4fb8dccfcca6231ad6970aaf710d15741a7437095f96e77a79f389bdacbc3e476efa65f0e46759fc0ea01eff9dc564730f41c1aced4b8f53b11564756fcecbb528f0e6132d700cd824d624f43a20fbc5c40e05d04068b1a5d01bb818d88f1b7c14b995d22a58f91b7129c2e6dac12aca400620ec55f21794f2da86d3b9d355b2627fd8e3bb6ee1aab260039917cbf22e09f6990cde3aeedc94556ea3edccfbac7d3706cb78114a300fbe1a28c8ee46524b124eb344ea755d7567b861cae53e24e7dc8d0f65f84009b0f1867cecba5f3538e6a2c834c3798a3550267b60ebdc1b0fedf5b7c5804b89b51dd264d990f27a5488d828d0c8179ada459bcb2f9212005940b1e93f2a418623f90c9e7d9a8bd0d7f92fed50ee9126f2c4c31315c99b9214bb4f55c0b340e40e67f9a0718949b9901016c4c5f15f161f5d9d08495e6af8a533398548892943ce1aedc4738947a730f61ff7c5535255d46801c817a8a02fa132e0be13abb297f6863b8400ba40132c7c6c408e45a1eb56da3e4e50c5b763963a71d3913f3a30e0ef1b76febf386c44e0e0665881c63aec320b8ecd47dec8f4ea969d2c9e81898e712fc77a542e49aab6946457e6b7e4896fc544f001b8f3d9c3c71345e8169497c89dfdac44fe23d05716b5e21545e6049bb0ec2591098d5402228b140869b6b0d01935346d424d122266b01b8387773d3e1a855ba39a5aed55f042809ca2a446b70ae76ed730fc5575749c2caf64f83e205ad53ab7b179531cf02da1ea64df3b826077e9df8a28ae54edeff3ba67b1fdc99ef3c7196d941c97b59d1769d62fb987f823bef6eb3ad928434ba2ae5d4978f34b8622da098693148da02ce7827c03d1acbf19837eacf21a972769632107091293aebad74f19b133903794b8ab14a91bce6abb77f829ede708d7f8265307e12fc5a4dede7fca0820e7e37bcc472aa10e819d68ff6b7e5148807868584ac588dcf7c3397edbd5ad87cd110c8749753d4de7074b4754d0a4b13e55ceb50ef2a9b4f0ecc2d3cac6b11d9a990bf5bde76c10efbf0368af4dad72eaf0420b1b523fb92be11b18c865380707cb08620bec97703b550b7c4032aea3bd63d9780ed3641c97da194d68a739d3d84a4eb315704aa01042cf61a65bacb3ac3ad7586cbc9f9b0a5bbf664bcf07fc900800fc44dbeef19cd96f3be771193521662febc95e611f67b2e0e8b0009836fd3a2e63a3e728d610d4e0b00140476f957c85c3a11822d5dca98c45dab0d1b17cb2bf6658cca95f0c0ad7fbc484646d5e0404ebc48a14d239b4277fa5ac296d59b26cdc9faa0d625c81c364e886f86653faac994c58d592140e90aed0c4d3533ff078edc1820f68f68c8c823ab65ddc15ec8d4e1b085ca8391c5b3e9208eb88053271e760a74dcad0cb31dfac7a084b6bd31427b2cfccbd742ff9fe5aead7be8754985b00de77bff4dce57ac62149a6a8ff86255a225bdfb925691d9feed97b40ee21d84207841ec2067a655c8178140dd6fa3771f2ca16fdcf023b913886c7e547c2d8980289afa21243478ac9efa41907118d851f1b64b2ecb1ed70532d4c6ead2ea37c726f2cf28485899f46254d3884a5d491b7ff4b5d4883181e04ba5587e5bdf225091d9da40738b9ef4ae8eec31f75c31a15cc7e18b5aa57beaa23b4aeb0165e0ca5a5409f442d7da63e9bc326f31d947615dde09b28c04dc5db24d732fd6de4a2f4368c380638fdd3c7609895d4bc4a0ce99196e37cd68fd248aac3ec9726612d24ad8153f635824797bf70f5f62f9e9268d4fdeb0b298a13ea58a3686b8d0756f37bc443e8b962b7aaaf620e1e36d2e4794c69d51c5c93e311dcea9887e4bfd4ec1cc55fd7b3f834fa67e9dd558be5ff46a234c2b60a7c3e09eef952157d5e6f1ebb98e67c58cd9e8feb800eeac802c1da8a6db45701613f325a12778944481d03429235690adc8f3c26f450f87a8070c31dddddb30bc1c9a1f7082422e0f9428648277edc56772771ccf15445b155c131b0a68ae9472038bbafc51811bd0046da0a4f1a2ccd52b0564fd1149e9aae55cbb347fb50d40ff30bf47a74a3e94da1b6e51925fdf4782de461e8c12f1549c4486b1dbaaf180f66b7a629166bda75ba3425041a27904145f37210058d716129a4225dc42069cd3c3566f891873f883363674e8f829c953f12951a2943d3d6e2d63ac462f8423cc921b8fabeaa1dd3a808a5a9ee46529921cd03c628c1796eae4909dea9cc67f5546c2fb2373273583ef5766b8921a90bce000f8ae3c763d106be6bc2c71755fe409496d6a15f9087fa6f2c4eb7097d5de63f7276bd9703cc698f57c7ef7cfcf318a92bbcad6d96b7865664433ea6aae427ca0ca91cb44ab92ceafdd78a867ec5e89ba90cbad9f5d41ac3d5a6aa8a299c1424cb81ac421fd8e9769756b69e61ae3f80599ac9efae9989b414ef5c6c9e03f83a9d67c501d33a0e55e2536652eca3307a5cbd249b2f630c462adcee74a9cce68680fb9abeb2d99fc41d4674f0e7353c9628067a86bc457bd12434ab00b9b8b2912c7d5bef82c8d7f0a811e9a36f38f1840c4dc3835e91e13bff3f729177e17dfcf6d55864f672ea5d536dad195e8bcf5c69bfeb56edb5ec289320308b52c235448536c5abeb991bf5873fa124d907e734dfef733f4661517209b7a1e40a90ca9dfdd9f28659915d203ef5c2e79b404b71774270f1a95d141f0852bfc424294ac23f0e9016aec753871b630d8a450f557103e648fd636eb41757e9e62aa03542f4bf6c169b18a80807179653dcbd616f7a02791f62d9b3a81b4963d893eae28ebfe7c6b50f14cfca29210eda3d0d2075bf89b5ea66bc638d1c8b7c6972089a1aea4b9eaefa43c24a7b878d124b6913bd3552f35a27e949183559922b9f1933c552aabd8d12581ecc9c1974886ec66efb08851097ee040601f5175ee609a4705fc11d7019d78ed9b38f2dee0334a8ebc30a4562c1b5c3d5a171cd869aeb8d428c4ee5fe05f72e40d5c461706bf1db150f9238eb1ba8f07ceb75d7f6ec8c22a27a79d753676f2f84ff6f86ceb7468077f0183c53215e6568eab521c2442ce4fbac1a81387955c5ea85bc627d193720cc868afaae981faca2891ea16c01b242ce69de63b5ed8c0450cf51eea4ddae82044587451345ad4cbb4b8084ad576f3d3af21eeb806b66d9b1ab58878618a9193c384905f10a2bfd123bc1c75ccddf8a8c0870f3b6dab0b04acc79509fb66a66d3ff48794c60376b8b7c0f9d1b07e84dd3c690858cacf189267c50088129afdf00c10e519d1afd69dbf7b7c594627abab3c8032090b011354296dae232107722c1e29428e05816216c9742a4922082ecbdb6bbb3d98bc0f1119f8caa6031cef77e1ac585abe90805d5d1b0eaddd28a25d6857a3e2c4625875e05d946b4ad58ea45c180bcd8f03993060e41cb9486b6ec99a500c3c0745eeae829f5f74bb7a4f0522046b9ab89280062804d56cd18fb5287a6e35288afaa9161cf76e47082df3ebd1a099b15263e5eebe71fd1080215d968143f6377718af48411b7cc28844ee47b9e9b6b8061cdc778c26d07bf2622db6401e8e2ed132f03a4954ab70532e8ffcdb043d03b2ec22d1e3332a2c6247273a2eaf3c99d93d9ccd9e29cbbd1437faf09bd66731b34bbbd68c7c0c0c1962164e1f5cc85a929a4c8774fba566cc91fd6f09052e886bc74e1225fe5f05c3e935c50e6ad625b86c99854fbd8ab7dddd1e4ea800145178e3e4aa0ca046651aa5333c3f818a47bbb0f67d71e3ab3bcd0b3d7147077818fc1937412f19eb382fa57c205c79d69b869362a47dc7335dc92ed4a3ab21c6caeaae48d643df9d16a20e3cecdde8b50b496fde404ddfad956a0791e9402627719781e4c5f659f1562dcd5cd1d4b480db075c0460ce2e4a131fa8a151baa4ef484bfe395df0c0cb2bc4397794c1436e4debfe599fd72d74c10af4d85f76d02442daf7ccc9d255a81533c88143e7a0dd27aa49a4e6d2c73776fcd75c8279e3154ed9f8819f1bbd92e74ca39579597ea7e3b5460ce114f6678d6565b7a868d93b9c2e0fa0d135f68a0c81a202f4254618ab1ef3a67696d6f7d7f93c8093f484d92b1d4f047575d606e7e81a4f728cccedbf20000000000000000000000000000050f151e272f383d")
	require.NoError(t, err)

	helpers.ClearCache()
	validators := make([]*zondpb.Validator, 2*params.BeaconConfig().SlotsPerEpoch)
	for i := 0; i < len(validators); i++ {
		validators[i] = &zondpb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}

	state, err := state_native.InitializeFromProtoCapella(&zondpb.BeaconState{
		Slot:        5,
		Validators:  validators,
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	})
	require.NoError(t, err)
	tests := []struct {
		participationBitfield  bitfield.Bitlist
		sigs                   [][]byte
		wantedAttestingIndices []uint64
		wantedSigs             [][]byte
	}{
		{
			participationBitfield:  bitfield.Bitlist{0x07},
			sigs:                   [][]byte{sig47, sig43},
			wantedAttestingIndices: []uint64{43, 47},
			wantedSigs:             [][]byte{sig43, sig47},
		},
		{
			participationBitfield:  bitfield.Bitlist{0x05},
			sigs:                   [][]byte{sig47},
			wantedAttestingIndices: []uint64{47},
			wantedSigs:             [][]byte{sig47},
		},
		{
			participationBitfield:  bitfield.Bitlist{0x04},
			sigs:                   [][]byte{},
			wantedAttestingIndices: []uint64{},
			wantedSigs:             [][]byte{},
		},
	}

	att := util.HydrateAttestation(&zondpb.Attestation{})
	for _, tt := range tests {
		att.ParticipationBits = tt.participationBitfield
		att.Signatures = tt.sigs
		wanted := &zondpb.IndexedAttestation{
			AttestingIndices: tt.wantedAttestingIndices,
			Data:             att.Data,
			Signatures:       tt.wantedSigs,
		}

		committee, err := helpers.BeaconCommitteeFromState(context.Background(), state, att.Data.Slot, att.Data.CommitteeIndex)
		require.NoError(t, err)
		ia, err := attestation.ConvertToIndexed(context.Background(), att, committee)
		require.NoError(t, err)
		assert.DeepEqual(t, wanted, ia, "Convert attestation to indexed attestation didn't result as wanted")
	}
}

func TestVerifyIndexedAttestation_OK(t *testing.T) {
	numOfValidators := uint64(params.BeaconConfig().SlotsPerEpoch.Mul(4))
	validators := make([]*zondpb.Validator, numOfValidators)
	_, keys, err := util.DeterministicDepositsAndKeys(numOfValidators)
	require.NoError(t, err)
	for i := 0; i < len(validators); i++ {
		validators[i] = &zondpb.Validator{
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			PublicKey:             keys[i].PublicKey().Marshal(),
			WithdrawalCredentials: make([]byte, 32),
		}
	}

	state, err := state_native.InitializeFromProtoCapella(&zondpb.BeaconState{
		Slot:       5,
		Validators: validators,
		Fork: &zondpb.Fork{
			Epoch:           0,
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
		},
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	})
	require.NoError(t, err)
	tests := []struct {
		attestation *zondpb.IndexedAttestation
	}{
		{attestation: &zondpb.IndexedAttestation{
			Data: util.HydrateAttestationData(&zondpb.AttestationData{
				Target: &zondpb.Checkpoint{
					Epoch: 2,
				},
				Source: &zondpb.Checkpoint{},
			}),
			AttestingIndices: []uint64{1},
		}},
		{attestation: &zondpb.IndexedAttestation{
			Data: util.HydrateAttestationData(&zondpb.AttestationData{
				Target: &zondpb.Checkpoint{
					Epoch: 1,
				},
			}),
			AttestingIndices: []uint64{47, 99, 101},
		}},
		{attestation: &zondpb.IndexedAttestation{
			Data: util.HydrateAttestationData(&zondpb.AttestationData{
				Target: &zondpb.Checkpoint{
					Epoch: 4,
				},
			}),
			AttestingIndices: []uint64{21, 72},
		}},
		{attestation: &zondpb.IndexedAttestation{
			Data: util.HydrateAttestationData(&zondpb.AttestationData{
				Target: &zondpb.Checkpoint{
					Epoch: 7,
				},
			}),
			AttestingIndices: []uint64{100, 121, 122},
		}},
	}

	for _, tt := range tests {
		sigs := make([][]byte, 0, len(tt.attestation.AttestingIndices))
		for _, idx := range tt.attestation.AttestingIndices {
			sb, err := signing.ComputeDomainAndSign(state, tt.attestation.Data.Target.Epoch, tt.attestation.Data, params.BeaconConfig().DomainBeaconAttester, keys[idx])
			require.NoError(t, err)
			sigs = append(sigs, sb)
		}
		tt.attestation.Signatures = sigs

		err = blocks.VerifyIndexedAttestation(context.Background(), state, tt.attestation)
		assert.NoError(t, err, "Failed to verify indexed attestation")
	}
}

func TestValidateIndexedAttestation_AboveMaxLength(t *testing.T) {
	indexedAtt1 := &zondpb.IndexedAttestation{
		AttestingIndices: make([]uint64, params.BeaconConfig().MaxValidatorsPerCommittee+5),
	}

	for i := uint64(0); i < params.BeaconConfig().MaxValidatorsPerCommittee+5; i++ {
		indexedAtt1.AttestingIndices[i] = i
		indexedAtt1.Data = &zondpb.AttestationData{
			Target: &zondpb.Checkpoint{
				Epoch: primitives.Epoch(i),
			},
		}
	}

	want := "validator indices count exceeds MAX_VALIDATORS_PER_COMMITTEE"
	st, err := state_native.InitializeFromProtoUnsafeCapella(&zondpb.BeaconState{})
	require.NoError(t, err)
	err = blocks.VerifyIndexedAttestation(context.Background(), st, indexedAtt1)
	assert.ErrorContains(t, want, err)
}

func TestValidateIndexedAttestation_BadAttestationsSignatureSet(t *testing.T) {
	beaconState, keys := util.DeterministicGenesisState(t, 128)

	sig := keys[0].Sign([]byte{'t', 'e', 's', 't'})
	list := bitfield.Bitlist{0b11111}
	var atts []*zondpb.Attestation
	for i := uint64(0); i < 1000; i++ {
		atts = append(atts, &zondpb.Attestation{
			Data: &zondpb.AttestationData{
				CommitteeIndex: 1,
				Slot:           1,
			},
			Signatures:        [][]byte{sig.Marshal()},
			ParticipationBits: list,
		})
	}

	want := "nil or missing indexed attestation data"
	_, err := blocks.AttestationSignatureBatch(context.Background(), beaconState, atts)
	assert.ErrorContains(t, want, err)

	atts = []*zondpb.Attestation{}
	list = bitfield.Bitlist{0b10000}
	for i := uint64(0); i < 1000; i++ {
		atts = append(atts, &zondpb.Attestation{
			Data: &zondpb.AttestationData{
				CommitteeIndex: 1,
				Slot:           1,
				Target: &zondpb.Checkpoint{
					Root: []byte{},
				},
			},
			Signatures:        [][]byte{sig.Marshal()},
			ParticipationBits: list,
		})
	}

	want = "expected non-empty attesting indices"
	_, err = blocks.AttestationSignatureBatch(context.Background(), beaconState, atts)
	assert.ErrorContains(t, want, err)
}

// NOTE(rgeraldes24) - this is not valid test atm
/*
func TestVerifyAttestations_HandlesPlannedFork(t *testing.T) {
	// In this test, att1 is from the prior fork and att2 is from the new fork.
	numOfValidators := uint64(params.BeaconConfig().SlotsPerEpoch.Mul(4))
	validators := make([]*zondpb.Validator, numOfValidators)
	_, keys, err := util.DeterministicDepositsAndKeys(numOfValidators)
	require.NoError(t, err)
	for i := 0; i < len(validators); i++ {
		validators[i] = &zondpb.Validator{
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			PublicKey:             keys[i].PublicKey().Marshal(),
			WithdrawalCredentials: make([]byte, 32),
		}
	}

	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(35))
	require.NoError(t, st.SetValidators(validators))
	require.NoError(t, st.SetFork(&zondpb.Fork{
		Epoch:           1,
		CurrentVersion:  []byte{0, 1, 2, 3},
		PreviousVersion: params.BeaconConfig().GenesisForkVersion,
	}))

	comm1, err := helpers.BeaconCommitteeFromState(context.Background(), st, 1 , 0)
	require.NoError(t, err)
	att1 := util.HydrateAttestation(&zondpb.Attestation{
		ParticipationBits: bitfield.NewBitlist(uint64(len(comm1))),
		Data: &zondpb.AttestationData{
			Slot: 1,
		},
	})
	prevDomain, err := signing.Domain(st.Fork(), st.Fork().Epoch-1, params.BeaconConfig().DomainBeaconAttester, st.GenesisValidatorsRoot())
	require.NoError(t, err)
	root, err := signing.ComputeSigningRoot(att1.Data, prevDomain)
	require.NoError(t, err)
	var sigs []bls.Signature
	for i, u := range comm1 {
		att1.ParticipationBits.SetBitAt(uint64(i), true)
		sigs = append(sigs, keys[u].Sign(root[:]))
	}
	att1.Signatures = bls.AggregateSignatures(sigs).Marshal()

	comm2, err := helpers.BeaconCommitteeFromState(context.Background(), st, 1*params.BeaconConfig().SlotsPerEpoch+1, 1)
	require.NoError(t, err)
	att2 := util.HydrateAttestation(&zondpb.Attestation{
		ParticipationBits: bitfield.NewBitlist(uint64(len(comm2))),
		Data: &zondpb.AttestationData{
			Slot:           1*params.BeaconConfig().SlotsPerEpoch + 1,
			CommitteeIndex: 1,
		},
	})
	currDomain, err := signing.Domain(st.Fork(), st.Fork().Epoch, params.BeaconConfig().DomainBeaconAttester, st.GenesisValidatorsRoot())
	require.NoError(t, err)
	root, err = signing.ComputeSigningRoot(att2.Data, currDomain)
	require.NoError(t, err)
	sigs = nil
	for i, u := range comm2 {
		att2.ParticipationBits.SetBitAt(uint64(i), true)
		sigs = append(sigs, keys[u].Sign(root[:]))
	}
	att2.Signature = bls.AggregateSignatures(sigs).Marshal()
}
*/

func TestRetrieveAttestationSignatureSet_VerifiesMultipleAttestations(t *testing.T) {
	ctx := context.Background()
	numOfValidators := uint64(params.BeaconConfig().SlotsPerEpoch.Mul(4))
	validators := make([]*zondpb.Validator, numOfValidators)
	_, keys, err := util.DeterministicDepositsAndKeys(numOfValidators)
	require.NoError(t, err)
	for i := 0; i < len(validators); i++ {
		validators[i] = &zondpb.Validator{
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			PublicKey:             keys[i].PublicKey().Marshal(),
			WithdrawalCredentials: make([]byte, 32),
		}
	}

	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(5))
	require.NoError(t, st.SetValidators(validators))

	comm1, err := helpers.BeaconCommitteeFromState(context.Background(), st, 1, 0)
	require.NoError(t, err)
	att1 := util.HydrateAttestation(&zondpb.Attestation{
		ParticipationBits: bitfield.NewBitlist(uint64(len(comm1))),
		Data: &zondpb.AttestationData{
			Slot: 1,
		},
	})
	domain, err := signing.Domain(st.Fork(), st.Fork().Epoch, params.BeaconConfig().DomainBeaconAttester, st.GenesisValidatorsRoot())
	require.NoError(t, err)
	root, err := signing.ComputeSigningRoot(att1.Data, domain)
	require.NoError(t, err)
	sigs := make([][]byte, 0, len(comm1))
	for i, u := range comm1 {
		att1.ParticipationBits.SetBitAt(uint64(i), true)
		sigs = append(sigs, keys[u].Sign(root[:]).Marshal())
	}
	att1.Signatures = sigs

	comm2, err := helpers.BeaconCommitteeFromState(context.Background(), st, 1, 1)
	require.NoError(t, err)
	att2 := util.HydrateAttestation(&zondpb.Attestation{
		ParticipationBits: bitfield.NewBitlist(uint64(len(comm2))),
		Data: &zondpb.AttestationData{
			Slot:           1,
			CommitteeIndex: 1,
		},
	})
	root, err = signing.ComputeSigningRoot(att2.Data, domain)
	require.NoError(t, err)
	sigs = make([][]byte, 0, len(comm2))
	for i, u := range comm2 {
		att2.ParticipationBits.SetBitAt(uint64(i), true)
		sigs = append(sigs, keys[u].Sign(root[:]).Marshal())
	}
	att2.Signatures = sigs

	set, err := blocks.AttestationSignatureBatch(ctx, st, []*zondpb.Attestation{att1, att2})
	require.NoError(t, err)
	verified, err := set.Verify()
	require.NoError(t, err)
	assert.Equal(t, true, verified, "Multiple signatures were unable to be verified.")
}

// NOTE(rgeraldes24) - this is not valid test atm
/*
func TestRetrieveAttestationSignatureSet_AcrossFork(t *testing.T) {
	ctx := context.Background()
	numOfValidators := uint64(params.BeaconConfig().SlotsPerEpoch.Mul(4))
	validators := make([]*zondpb.Validator, numOfValidators)
	_, keys, err := util.DeterministicDepositsAndKeys(numOfValidators)
	require.NoError(t, err)
	for i := 0; i < len(validators); i++ {
		validators[i] = &zondpb.Validator{
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			PublicKey:             keys[i].PublicKey().Marshal(),
			WithdrawalCredentials: make([]byte, 32),
		}
	}

	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(5))
	require.NoError(t, st.SetValidators(validators))
	require.NoError(t, st.SetFork(&zondpb.Fork{Epoch: 1, CurrentVersion: []byte{0, 1, 2, 3}, PreviousVersion: []byte{0, 1, 1, 1}}))

	comm1, err := helpers.BeaconCommitteeFromState(ctx, st, 1 slot, 0)
	require.NoError(t, err)
	att1 := util.HydrateAttestation(&zondpb.Attestation{
		ParticipationBits: bitfield.NewBitlist(uint64(len(comm1))),
		Data: &zondpb.AttestationData{
			Slot: 1,
		},
	})
	domain, err := signing.Domain(st.Fork(), st.Fork().Epoch, params.BeaconConfig().DomainBeaconAttester, st.GenesisValidatorsRoot())
	require.NoError(t, err)
	root, err := signing.ComputeSigningRoot(att1.Data, domain)
	require.NoError(t, err)
	var sigs []bls.Signature
	for i, u := range comm1 {
		att1.ParticipationBits.SetBitAt(uint64(i), true)
		sigs = append(sigs, keys[u].Sign(root[:]))
	}
	att1.Signatures = bls.AggregateSignatures(sigs).Marshal()

	comm2, err := helpers.BeaconCommitteeFromState(ctx, st, 1, 1)
	require.NoError(t, err)
	att2 := util.HydrateAttestation(&zondpb.Attestation{
		ParticipationBits: bitfield.NewBitlist(uint64(len(comm2))),
		Data: &zondpb.AttestationData{
			Slot:           1,
			CommitteeIndex: 1,
		},
	})
	root, err = signing.ComputeSigningRoot(att2.Data, domain)
	require.NoError(t, err)
	sigs = nil
	for i, u := range comm2 {
		att2.ParticipationBits.SetBitAt(uint64(i), true)
		sigs = append(sigs, keys[u].Sign(root[:]))
	}
	att2.Signature = bls.AggregateSignatures(sigs).Marshal()

	_, err = blocks.AttestationSignatureBatch(ctx, st, []*zondpb.Attestation{att1, att2})
	require.NoError(t, err)
}
*/
