package remote_web3signer

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	dilithium2 "github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/go-zond/common/hexutil"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	validatorpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/validator-client"
	zondpbservice "github.com/theQRL/qrysm/v4/proto/zond/service"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/validator/keymanager/remote-web3signer/internal"
	"github.com/theQRL/qrysm/v4/validator/keymanager/remote-web3signer/v1/mock"
)

type MockClient struct {
	Signature       string
	PublicKeys      []string
	isThrowingError bool
}

func (mc *MockClient) Sign(_ context.Context, _ string, _ internal.SignRequestJson) (dilithium.Signature, error) {
	decoded, err := hexutil.Decode(mc.Signature)
	if err != nil {
		return nil, err
	}
	return dilithium.SignatureFromBytes(decoded)
}
func (mc *MockClient) GetPublicKeys(_ context.Context, _ string) ([][dilithium2.CryptoPublicKeyBytes]byte, error) {
	var keys [][dilithium2.CryptoPublicKeyBytes]byte
	for _, pk := range mc.PublicKeys {
		decoded, err := hex.DecodeString(strings.TrimPrefix(pk, "0x"))
		if err != nil {
			return nil, err
		}
		keys = append(keys, bytesutil.ToBytes2592(decoded))
	}
	if mc.isThrowingError {
		return nil, fmt.Errorf("mock error")
	}
	return keys, nil
}

func TestKeymanager_Sign(t *testing.T) {
	client := &MockClient{
		Signature: "0xb9543b1ab4c4239af0c3be9cb0591105dd8fed55851e8307b5808b2417325930e3ab2b80a7f26c23e7a7fa5c63aac9714e296a2f718cdb8fb0b617e0e4d6b71e439849a38983c1248094cbd48ff288800afa2b92a3aedf9a348b125ce47a43d95848e64fcc0596dad9bc020d7f75b6d2df0440e151ad11af83efc660ab0d28d6993d1d653b63ee08680fdd101a1458009fb8cba54ab2ffdab6bb979f8a5aefcb82acf224790784f01d12f0541f512cadfd7f876ec5f410d8e4e0a92d6aa8ab453680707786f2315c4be934882151b7fd82fb59973fe1bacc3724efc5683755536e0c537385c9c578363dd1bad686312e5234e036ea25b42f4964aba52333eb64e1a71e90f7a5e7582b5ac20f4866aa884ef75b398df0e7cd00df76e91fcf310864f6d86379cb2cb255dbeecf66eed63e5fe87d442fe2bb84f63ac54f626ab853ddc57ae1b736c4adc5403bab85433ce1b137071863e87467f9ca97b2b1f407bc88f16d64dd8ca1e628b7472607fa5641fa268e96c85a28c30a471d8554ffd4b6a5cb474ef06c9c1c5be8bd28993266f77238a7f5a030ee66c8a2d8e8e29fdbef5e8a08f95f37748fc714ff0b6fc9f94acbb75044787b1498da110dc4460932011bd69ac5675a426065d30dc3de3babe0a5770771e466d7a9c6b393a0d6105568a29c3f74bde371b3b6ed0dc6b1eee5aea5a98df692b66a94f0a9c192691ae43046f1e370d5b39ad9baa1726ef2e8f5a823522aae2cec164da4a1f00bf12a69d3ffc8f01aa30bcb59c058bc4b383d2b1e547dfababaa95f4627d45a66fadab3b82205648a9567193549525541cc2231a78c6a28f61b964356b2cf40490994132d55d66dd801433bafdb758435e295b5225ef6ff994b8fc16714bda1941a64a73a5ace51e5d8aedff59c71165b82e370d70551b08a456f6b5e178f0d256f390dda839d6ad6e4e2c6caae6b504c70b8e3ea7719df9916c7d3ec92db4cc9e3f9b7d4884faf7ba0675da20f819fd24608be1a1873c5ed0c4c313e239444a0df59a194724d4cda7b94e7b74e41056e2ddf5034abd0460137b58fc2f5e413bfe5ebd026c473dff1693398b1917e6900db552235fe578593a8230e08d70452c92480a20a3234e8941e02eaf086b0c4ff428e2a61a618a1428ee22be562bd8b0cc200b625575e7955768d7514be79141d80fdd7bc7ee006d983dc2a992b23dc2276b8ed4d6cc58645201e7b9bee701e4a12c74976b54f5de1a7e701bc9d7c6c79a1fa56fb486d500583f249e77a7c49ce1339633bfd5375b22a4bce5a45dd273bd3650a8cfe31b96b4368f6d69cb93b9d267a940f13b4d0465cc32b4757df6d82d789021b1f0a66e0cfe69eb866b60d67548779ad430d9301ba8b2b746cd91e32eadca63afb9c410efb73c6bf820c8e1dcdecf1dbfa6fcb31781a8e4c1b9541ec040dc2cd6510a6df3afb5124a308feb4b7ca5268ca352979260a8c557951b77bb153bf87fba26bff36464268f7b290bf0f4f7f91a798e60bc99dbaac9626302a3edb605971a0e22b968fec04948df232d6244839f9b63aca524b257da6e46638f4744eb04a669d97ac483f49548273a225042bdbcc1eae940bb54d535448e9aab6f1ee5c73650be76b42f2aa3c68d4abbd00e9cfc395e1059439914d742bd0344aeaee44e9d88119fe3466570300e354b697fda30e3ea6fcf591cf4d50750afe73dfacfd54c99b574d05fadd9e83aac32f5a29d7df18f6dab3d0930ce83d05cd9d74e71da615f15eb6aab9254821d45eaa126344ce85bba1aea6001c08187d70a07bdddcac956debbcdfdc61cbfcbf303ff3d9fb1f475094ee6e56015e45b93dfde6fdfd67692369ad5641e3fae3c59e6969f0989ef90b21c9d6119cd938b8b23ff7f4b92b50efc676781da94dbe11b912e34791df9555642de222323f197e8d297ef483b88c6ebb630104e19733d820a6b53a6f731af9f43d53de4eeba0784166bb86fbddae21fd4f15c951183a1e66a70ec5b0e322038328360dc3ace2f22c906e9b6deb77cf9583fcad826e0a32486f43d72f94e55403e53e0e3e737b4cb1bd2960535ab1102ec60185f14eec06aa71315eb0b2277074c6d87a8f30ddfa7c50da23028d19066a2cbab4eebf8b979eb4d6160c5030272f0d9469da5b813bd59e736842b1437e6c1dd5b51e2b7d98c09d48f3e5cbac761c16b6be7cb953b405791b4fc64c1516b2ae6c6df73909ff577afe7d4667ebf076c708db30fcf7b8866f3b29ea63faa397e6a1c5ddca2dd26ba1e17796ad36ee9ed1c4bb377bb0613d79de7e6db2358f4e88e72b2f6bf463e6c434b6d19319eed3938708d5a28ca29c26eb22d8c9fbc907944e35f108821519710fc8aef732104ef4494ce69e8e3e47b860a2bbc87245999a87985b5c2d2482785d273996861516c4e847f48c7300fede41a847b36c8b3bacf9f4b4e2fdeae0ff7d59a878954908835fa213258faf0ede52b4010f04fa681fbb42600027165d6dfb8fae5c2d8a7abdf2fa4e455eda3400c31b9eb9da71d5ca3980f188d76e2989b0b6ab33a7845953adcc1c9ff689fcc4cf77408999921f846e294a479b80ca4d0d7ac3aafde656b822f026fe1eebc380c66ae167a8eb8387f713b3824b7c79039c41f31ec0b96d16087e8247af932caa4e3d591f8a249455eb8364e291cf10223958c37e8c78545c015d9de8f92863114ddd26f692829c35d3d095493b0aa8354e7581691ea8953e006f502841f53c7d46ff047f1f5a8f220e8c9edddb49373c42fcd7df427dbef647d54485cb034136d99c17d37a12197c209061d18d525a0f9bcf06f863084bee16893756349959b9ba80612a0a2dd137f3fbe78566d78ed828df3a65d3c2a1c7c9f63b86d4dc6925332ee4fcf7c4b3c2a662114be77e3ec2241332898a71fd9dd29a05fb107cb43388f672f0d066a0a5255b44722bb8f25eb1bb06d2e3003fdbab56d171d31772f93f5d20f150a224c2d101ee0f4c830d6a2255678dde5d9c36ffc7525cd519f0184a666b96543f59c88642f6a104526123d64b7755d459cefb3c16b8f7231f3935f846c17ce30c1222cd3808dd3b1532acc71bec0979fd1e23e93f4d492ee48bcb066429a20710a576482e5ceceadea8b60cd8f16c3c5a39b3fac089ea8b887d0e674a5391c55a254a9b52a6da50ad4e7b91184569ac68e7fad6849ca6f643dc5a6413c0066e76cab60db50e3e17b3698e81f19cec2a9b7c1a615243075a421382bdc782d4fc0bcb1c018b65f2aa79274f52a1b8c7ed70216075cc70fc6b5500b48141d02d45c5a404193657b7d77580090486043291ef6beba631939770924ffe76cd795fde94c66422281888ef29ff5409cffef5ae2868a34b04762e9f3e1b960a8d83215f68973e8974a8bf888e8dbd00d9a47a033b58c2f51c06cdc7162feb76fafa885ff1d2f5d04d951524d84728127f22f4b29c2f486af4ee831a2a59ec7bfbd84841f87737bd86a9eb99a15bdd15933bd85618c45478ac29b9fe126567309ca8abcb25a3382d002b578c90d6b9247c60f89502582e59872ecdaffc97478d5e82cc962b15125e2180776b63ff2758ab231e0179280ff3c17c0429bdd7df8732fd0ab200cda47e318f324a9dcdc871864d708b4368e603a715a1f0daa4ea1d5b12c61322c7935e39679f4c4088737e9fee42579689902c97b3a0f437943cd80d6dd2356db798a7e171856e8a865d038225890fd990ccc4dba21e5928670a53384df172839a12662f97a0f2e90977e5f978017a381e8e2addc5dfe8d00c7d8b8da8a98e13354069b5b15919d23e621774b0dcc02bd2243c07c18f39e8d55765bebfeb5cdfa265b6fdeb63f6848b691e4942de6732f6beb7e6f234ba2cb608cef0d3560f631f409403f029ecfd15d3da8067222ddfb25185b9ea8a2fb5204582ef00fa3df096f3179f68bb64dc446c5c2c663e1b18b9afdcdccc5e0220fbf3fd96a94bc1b4d4e0a256bfa2e204cbc32e997425799cdad9f594497a6f2204f2825e1833298c0d856294730d7aa058301fedfbfe81d2cea4d20640c2979aed5aac9e066c6b13982a4f7c454c4c9a46a32f71a035087b052b8dfc14aa2ccbc6c7fb9732ef36fb227fa24cd337205b545c25b243fd7467d8e1aa585ac8c47e443a45c41ea00bcdcd2076066caccf9ebd950d5e2f1687851924790895b07ae02728b5a665707cd404012956a77cbaf4db54cd2ce03f1b6fc4fd0262a0f47190dba7cc78f493cb5acf0a7a5a7027caa5babef6f02aa3dc43fa105fc3ab102ccaa178e720c386c57b3ea3f78ada73de4b16f5d5a2ee258f332e54c530f1d7a98a21e520d5b3dc3b98970da9c9c538c34e066cbc10c3647d913cab6147fe1fdf06b5c041ef5aab66da6dbac0ea3907e8fb2c96f5be4c1c3e07bc243b34eb0ce7e919117ffb612aef84b5e40c8a3efe5d71f3610b158f0991715144226495585fea493f39aa8d20b84883ecd7f270e93cb0940def7338f16e455b6f828986b3ab00d1b97fb3f731ee8df7c5b794f1766fee03c29e8527778c0907a7d372dcf5646a55da10a3a380046c8435456dd2abc2e78293c13be2a6bd61410769fb29202c93f723edbe0021449594c3f634cdd9339b73c9d18109325ed873a3f0914fbb4bb00e9a72ff8860c707cf8a9c828556b7c9e5af2d886575beefed7e4fffc6d224991ff4cde2ba7484ec97ad6690ffe74aa6dd7f101e7731e744d6dbdfe7dcd2f09dcf5b81c135ec2635d2497471913949c1e0272cbae894841e33e5dee70cb9af86d8d15656649c3bd05643d55afd1cff5d6ee3e790491597f105d3bd972fd64a5cb0cf850fba521a2c686729a9fca18cd172dd0528b619824dcba0f311ddcea4c4a3625281d61aba813aaf7f3d2c16488043d9d23189698a8fad9023bb615fe8796cf1fe0f09330c7e0b03e433d326420a7887da237f7a57f5582d0fde2f45e501d63020320d48e037cdf4c2a9782d2fd27dbc8cf36a7ce3303da3dbfcd032e5989698b85a08212b9eb3978408a0dd8f88327ea7730afb51ad3517e2222df5ae78f9025b2824efbed445bba229945c0055ad4bfd359b129dbce302b05e9ed0eeb3ae8341d78646c27cac791b09f5ecbccd85e6b8a515fc9da521537af2cab14017bcb54e77c793d83d8a2a85eb32a79d4ea6e1abd00610364f3b77b167cbd5507bc0b9ede0b2314fcd1117ed114d0687aeca33affdb4786ce544be01e407e02d5a2c11df4bbe69044e19a33a09ac9d8427e798c145ea692fe078747e78f9e71498c606ff3da744fcf83f30778562f71bd5d7ff468e2f5ae51988e85e89fd30f46e187542df80684bfddd48e264b8e2517ea104207ccb70fa08a399baf92f7e821bcb74a7432faa44fde851931645ab8de3161c061974b53e2cda757e056ea7f80e10a380578022277a38ff3f0fda8c8c01d5a75a893b0aaecee26aa30b3a126fd3ab775977c10c2a9a96f161631ef397014380e4557cece328fbcd0e6a78418811eca8fa48459f48c48204e65eb7b5e15669c9b53cbb9b44ee78da8283bfe0ad655ad773e4102399ef66902cb1106c80668d59b99cae90c5b667d88e2be31b856afffadbff9fe3f66e9be727a59c6632a7895bd610f9c1a3c8ad8cc004d46e1f0532601550dc9fd7e3dc4bbd5bbd9c8654bc4106a28cae2e269fcda3d8217f520331a857ece842229c82822dc59a8c48e7d39acf7f3dcf49feb357426aa6304a1e0115e65f8b64f4c230cea7c6d138c80638429b0752e11c81067fe0bd57c7fbde5a725f900e509ff1f008b891e5bf37706610a323b08aa5e5a8b6977c988b48b37ada0782900882ee012021161b945aafb3146b0ef3a3cae6d33e0c93b89e77bb31e30f3e6694edaf6a8e11c5c3434a18521746bd5c0fbbc0b290223e500d0190406e3084a7f144d567a59a25b459110a6e9ce1bf1d8f6acbefab12f85e52f5af5f82f11c0d036683a6fcde84a600fcc2a7719140b4932696c76b81dfb46b127314488c4ec565c14c41613ac1df2306f6e7d0ff175c5df4dfaaf85f5ddec22782a5512975c90d76feb474f78a4edd5fb1d4898fc18d32e38f06b98841539113405804848d6f8273a987dd1be2468bf516062c8311242e3ee8e0cf9c6134dfa11fd98b1dec4c89bcb57d0e1e10ff0c591374a4d6c86b3390ff9fa33aa03befa46c4e7c78e956cc6d519eb766ccc722124fe025fb79cfba93863eff199fe06d4c515aeedd015f9d7527261f4feb4ac1abf26769e16101b7de74bf5f9f61810325c025d86db20f5258f656334ca755c422fc653af0480e10e17b495fe31820b4add945d9c9fa61adb81c5486bced715242bcf9eb62e26f7de37e85796428e0e9a83a74e8c624370c5a5e8aa7b0b305286f8f90c9254272f71d23373d41559498b6d1eb0e3e49585b647f8db4d5fc00586d9ab2b7e54574a4dae222313f65c900000000000000000000000000000000000000070d111c272e3338",
	}
	ctx := context.Background()
	root, err := hexutil.Decode("0x270d43e74ce340de4bca2b1936beca0f4f5408d9e78aec4850920baf659d5b69")
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	config := &SetupConfig{
		BaseEndpoint:          "http://example.com",
		GenesisValidatorsRoot: root,
		PublicKeysURL:         "http://example2.com/api/v1/zond2/publicKeys",
	}
	km, err := NewKeymanager(ctx, config)
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	km.client = client
	desiredSigBytes, err := hexutil.Decode(client.Signature)
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	desiredSig, err := dilithium.SignatureFromBytes(desiredSigBytes)
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	type args struct {
		request *validatorpb.SignRequest
	}
	tests := []struct {
		name    string
		args    args
		want    dilithium.Signature
		wantErr bool
	}{
		{
			name: "AGGREGATION_SLOT",
			args: args{
				request: mock.GetMockSignRequest("AGGREGATION_SLOT"),
			},
			want:    desiredSig,
			wantErr: false,
		},
		{
			name: "AGGREGATE_AND_PROOF",
			args: args{
				request: mock.GetMockSignRequest("AGGREGATE_AND_PROOF"),
			},
			want:    desiredSig,
			wantErr: false,
		},
		{
			name: "ATTESTATION",
			args: args{
				request: mock.GetMockSignRequest("ATTESTATION"),
			},
			want:    desiredSig,
			wantErr: false,
		},
		{
			name: "BLOCK",
			args: args{
				request: mock.GetMockSignRequest("BLOCK"),
			},
			want:    desiredSig,
			wantErr: false,
		},
		{
			name: "RANDAO_REVEAL",
			args: args{
				request: mock.GetMockSignRequest("RANDAO_REVEAL"),
			},
			want:    desiredSig,
			wantErr: false,
		},
		{
			name: "SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF",
			args: args{
				request: mock.GetMockSignRequest("SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF"),
			},
			want:    desiredSig,
			wantErr: false,
		},
		{
			name: "SYNC_COMMITTEE_MESSAGE",
			args: args{
				request: mock.GetMockSignRequest("SYNC_COMMITTEE_MESSAGE"),
			},
			want:    desiredSig,
			wantErr: false,
		},
		{
			name: "SYNC_COMMITTEE_SELECTION_PROOF",
			args: args{
				request: mock.GetMockSignRequest("SYNC_COMMITTEE_SELECTION_PROOF"),
			},
			want:    desiredSig,
			wantErr: false,
		},
		{
			name: "VOLUNTARY_EXIT",
			args: args{
				request: mock.GetMockSignRequest("VOLUNTARY_EXIT"),
			},
			want:    desiredSig,
			wantErr: false,
		},
		{
			name: "VALIDATOR_REGISTRATION",
			args: args{
				request: mock.GetMockSignRequest("VALIDATOR_REGISTRATION"),
			},
			want:    desiredSig,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := km.Sign(ctx, tt.args.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetVoluntaryExitSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.DeepEqual(t, got, tt.want)
		})
	}

}

func TestKeymanager_FetchValidatingPublicKeys_HappyPath_WithKeyList(t *testing.T) {
	ctx := context.Background()
	decodedKey, err := hexutil.Decode("0xa2b5aaad9c6efefe7bb9b1243a043404f3362937cfb6b31833929833173f476630ea2cfeb0d9ddf15f97ca8685948820")
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	keys := [][dilithium2.CryptoPublicKeyBytes]byte{
		bytesutil.ToBytes2592(decodedKey),
	}
	root, err := hexutil.Decode("0x270d43e74ce340de4bca2b1936beca0f4f5408d9e78aec4850920baf659d5b69")
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	config := &SetupConfig{
		BaseEndpoint:          "http://example.com",
		GenesisValidatorsRoot: root,
		ProvidedPublicKeys:    keys,
	}
	km, err := NewKeymanager(ctx, config)
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	resp, err := km.FetchValidatingPublicKeys(ctx)
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	assert.NotNil(t, resp)
	assert.Nil(t, err)
	assert.EqualValues(t, resp, keys)
}

func TestKeymanager_FetchValidatingPublicKeys_HappyPath_WithExternalURL(t *testing.T) {
	ctx := context.Background()
	client := &MockClient{
		PublicKeys: []string{"0xa2b5aaad9c6efefe7bb9b1243a043404f3362937cfb6b31833929833173f476630ea2cfeb0d9ddf15f97ca8685948820"},
	}
	decodedKey, err := hexutil.Decode("0xa2b5aaad9c6efefe7bb9b1243a043404f3362937cfb6b31833929833173f476630ea2cfeb0d9ddf15f97ca8685948820")
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	keys := [][dilithium2.CryptoPublicKeyBytes]byte{
		bytesutil.ToBytes2592(decodedKey),
	}
	root, err := hexutil.Decode("0x270d43e74ce340de4bca2b1936beca0f4f5408d9e78aec4850920baf659d5b69")
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	config := &SetupConfig{
		BaseEndpoint:          "http://example.com",
		GenesisValidatorsRoot: root,
		PublicKeysURL:         "http://example2.com/api/v1/zond2/publicKeys",
	}
	km, err := NewKeymanager(ctx, config)
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	km.client = client
	resp, err := km.FetchValidatingPublicKeys(ctx)
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	assert.NotNil(t, resp)
	assert.Nil(t, err)
	assert.EqualValues(t, resp, keys)
}

func TestKeymanager_FetchValidatingPublicKeys_WithExternalURL_ThrowsError(t *testing.T) {
	ctx := context.Background()
	client := &MockClient{
		PublicKeys:      []string{"0xa2b5aaad9c6efefe7bb9b1243a043404f3362937cfb6b31833929833173f476630ea2cfeb0d9ddf15f97ca8685948820"},
		isThrowingError: true,
	}
	root, err := hexutil.Decode("0x270d43e74ce340de4bca2b1936beca0f4f5408d9e78aec4850920baf659d5b69")
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	config := &SetupConfig{
		BaseEndpoint:          "http://example.com",
		GenesisValidatorsRoot: root,
		PublicKeysURL:         "http://example2.com/api/v1/zond2/publicKeys",
	}
	km, err := NewKeymanager(ctx, config)
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	km.client = client
	resp, err := km.FetchValidatingPublicKeys(ctx)
	assert.NotNil(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, "could not get public keys from remote server url: http://example2.com/api/v1/zond2/publicKeys: mock error", fmt.Sprintf("%v", err))
}

func TestKeymanager_AddPublicKeys(t *testing.T) {
	ctx := context.Background()
	root, err := hexutil.Decode("0x270d43e74ce340de4bca2b1936beca0f4f5408d9e78aec4850920baf659d5b69")
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	config := &SetupConfig{
		BaseEndpoint:          "http://example.com",
		GenesisValidatorsRoot: root,
	}
	km, err := NewKeymanager(ctx, config)
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	pubkey, err := hexutil.Decode("0xa2b5aaad9c6efefe7bb9b1243a043404f3362937cfb6b31833929833173f476630ea2cfeb0d9ddf15f97ca8685948820")
	require.NoError(t, err)
	publicKeys := [][dilithium2.CryptoPublicKeyBytes]byte{
		bytesutil.ToBytes2592(pubkey),
	}
	statuses, err := km.AddPublicKeys(ctx, publicKeys)
	require.NoError(t, err)
	for _, status := range statuses {
		require.Equal(t, zondpbservice.ImportedRemoteKeysStatus_IMPORTED, status.Status)
	}
	statuses, err = km.AddPublicKeys(ctx, publicKeys)
	require.NoError(t, err)
	for _, status := range statuses {
		require.Equal(t, zondpbservice.ImportedRemoteKeysStatus_DUPLICATE, status.Status)
	}
}

func TestKeymanager_DeletePublicKeys(t *testing.T) {
	ctx := context.Background()
	root, err := hexutil.Decode("0x270d43e74ce340de4bca2b1936beca0f4f5408d9e78aec4850920baf659d5b69")
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	config := &SetupConfig{
		BaseEndpoint:          "http://example.com",
		GenesisValidatorsRoot: root,
	}
	km, err := NewKeymanager(ctx, config)
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	pubkey, err := hexutil.Decode("0xa2b5aaad9c6efefe7bb9b1243a043404f3362937cfb6b31833929833173f476630ea2cfeb0d9ddf15f97ca8685948820")
	require.NoError(t, err)
	publicKeys := [][dilithium2.CryptoPublicKeyBytes]byte{
		bytesutil.ToBytes2592(pubkey),
	}
	statuses, err := km.AddPublicKeys(ctx, publicKeys)
	require.NoError(t, err)
	for _, status := range statuses {
		require.Equal(t, zondpbservice.ImportedRemoteKeysStatus_IMPORTED, status.Status)
	}

	s, err := km.DeletePublicKeys(ctx, publicKeys)
	require.NoError(t, err)
	for _, status := range s {
		require.Equal(t, zondpbservice.DeletedRemoteKeysStatus_DELETED, status.Status)
	}

	s, err = km.DeletePublicKeys(ctx, publicKeys)
	require.NoError(t, err)
	for _, status := range s {
		require.Equal(t, zondpbservice.DeletedRemoteKeysStatus_NOT_FOUND, status.Status)
	}
}
