package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/prysmaticlabs/go-bitfield"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	types "github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	v1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zond "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
)

type roundtrip func(*http.Request) (*http.Response, error)

func (fn roundtrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestClient_Status(t *testing.T) {
	ctx := context.Background()
	statusPath := "/zond/v1/builder/status"
	hc := &http.Client{
		Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			defer func() {
				if r.Body == nil {
					return
				}
				require.NoError(t, r.Body.Close())
			}()
			require.Equal(t, statusPath, r.URL.Path)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBuffer(nil)),
				Request:    r.Clone(ctx),
			}, nil
		}),
	}
	c := &Client{
		hc:      hc,
		baseURL: &url.URL{Host: "localhost:3500", Scheme: "http"},
	}
	require.NoError(t, c.Status(ctx))
	hc = &http.Client{
		Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			defer func() {
				if r.Body == nil {
					return
				}
				require.NoError(t, r.Body.Close())
			}()
			require.Equal(t, statusPath, r.URL.Path)
			message := ErrorMessage{
				Code:    500,
				Message: "Internal server error",
			}
			resp, err := json.Marshal(message)
			require.NoError(t, err)
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBuffer(resp)),
				Request:    r.Clone(ctx),
			}, nil
		}),
	}
	c = &Client{
		hc:      hc,
		baseURL: &url.URL{Host: "localhost:3500", Scheme: "http"},
	}
	require.ErrorIs(t, c.Status(ctx), ErrNotOK)
}

func TestClient_RegisterValidator(t *testing.T) {
	ctx := context.Background()
	expectedBody := `[{"message":{"fee_recipient":"0x0000000000000000000000000000000000000000","gas_limit":"23","timestamp":"42","pubkey":"0x94ef47878aea6c24a6aac5d43465cc361bbaf8bc8c9eba9abccda48977767f5604b8150337fd5ca7cf90bf8f63fca0e6fc0728a3071e5ccae2766a15679d2a57ddc95f6f51ff8bb831aaa937271eb80d599566ae1e73173aad708f68330bbd9c6555c0f9366763011f7aa4edebab101f99a4007c8ae1123a13e7c7cc19e2e7699b549bb770d3753bea49ec9e31104bae89fc38abe75e1f140267a2f492409d25f188aec4783afd2c8140f6a8c6850077536cb2760c65779d165b6f03f9b149210d8160f58803d31171be717baf887aa612c02f806bd9e3332ddc21c0e6e912d053d4d49c13d2de8a75266e6157610175d4897e58886aae12bf7b949e20351d80a4a994e7c70c9ba76a2472818343609061ffa393f6f270dc8b4aa806d5616e55e936f26cccd3e1006bf185196ae5457d224fca6555068bfb64a228e8b44b4221e85d2f1137224992f41a78301ff768527e953e50424a45d21e8877a16a915629f45aba1aca08769c561260d4d58bcf36af98c26e6b81365b91720f3155c1f9383d8c7ab295aabe6c9799f625d29da42bd4002fbd337ceaf991573678f6384da18541c4e3a3c9472620c19129bb41e2b5e71884b98a8ac319a0ec2be11948f4c02b0b824a2347e00945ec89f45f431dfb3605d8228ce69136867570bd0149f4fe2b53f19d1458e7d6f9688af7c5ec3021fbe61cb331a5a44c9f5e9a4a1192d5eeddf98f47e1c8379bf000ae6886cc3eb5442fa8586652550876a4ef31dd941eac082e9fb1ff26db706159ec7be0a5051b408fc955c55335db1f46c6e87113aabb03960f2f4fc986e4e583021b6b69e7c68b0d1093429630cf7f4a7e895ad45a41363c53d0ddce0804a8858092bc7a069852fff02773e0abd6c7cc7d3d34c0bebfb34e1c5e95ddb184c4a0fc77ed09fca96dc472ba0391214c489890ea410085d4c6ecc69f3facf0b1587372752a421641597563fedc9fc64452d7a7a0db9560ec1c8564a3180a5e623b65e1dd494f967556bde56f9bf58cf5e07a050258e4a90cc2700831a4113391877c65ebaadea8710e23ca9f7afb8c5bd90edf38211b32874b65bef5455159c1a1d17ac3da17819f8adcb254a62d7f4c362cc470f75fddaf6f6624f3a56b0187c4e21295534a832a0f2720c411d0685751fff095078d18bc854856f7e1abe14eb76e9e45fd1eca282804784d9c27fd15ab00bb6dbfd864c401f759d0f2da8ad8640b81066038b72c6cf26605f8388cdd67aeca21dda79d6dd01bc3d3ff5ee29f5e016ea681ba581940d0130685d42e9635c6ca27e1eb9fbf08f44879f4f479eefbde7b65476fbe379f771b0f116a2e6e5f65416b72f2c49452c40b2f108ab86ac7dbb8e252ed32946e5be280b512734c96db9511b21eeec3d0caeac51f8ca315ad7dd62efd1113e03932856d5eef73035a0ae24fef22a2c8aa1db28bc87702c34b2d2d722ddf0ad9d2eb4a6c16b85a7e22c49d115c676afeef8f66ec95ec805e4c1423df5dc2eeece55f107005e15b3ded7c5ff7b7d6fd8530049ede7b776bee594fc18e29a4ae177419bfcb2a0185f51ab35c389baf9b37742192e1c36c2ba7d5c46f683dfa8ca8103a824033c68d844d0ed55e560b96b8421162f57e2daaef5853bfe476903fcb42196e78ae2afe3d5da230de9628e2d268dfc50290b2a8ddae0ade58e2919e0f9be4b538220b885e20b28741cce7c6072aa27cf076197d8e046072045f4a4e20058e7ff428419ce7629b7f76dc0568d99586fc4095107b02228d4f2e9978abe68ef302747a67805d0158b88e94bdb73f1c6e1ca8918b311b0a5d101130e142d2085778d1546532c842f2673cb7a3774eb6d23fa901883aaa682aa7ed0195c3f899f6b485e09715a3791ac9102c177b37465344523fef3e9479ce39d358db0106b5b4cfe26415c5ebc2d00ab36af1171ca33f2c95ff16b63f91e10de4405aec7d6368afc20643616bf30505507f7af84a6f70a36c9f6644bf22bb141f88f15c01f99fbb0344b6db03c042cb8e80e6e38717c3e749cae3782b3d3529cbcfd68a04ade376a7f334ea471d21ebbc62035a57cced3f74a1612edb492a13aacc93d51348249451616f01bbd0e89f46fa53acc5490c7164ef8b2ac0a236b9da37f696db7d2e2dd51243966d2deeae418f2edc6f38fabe746ec5bc832db5af6856266c140e78ce15c2699778910c8e002290b52a68bd14c3be294154c7f448be0160d4adc856b307aa4d3a3ec82af0d25d951d25dc2027ed7861c9ef7d0228179a9fd38ba45f8721d6ce3dd42dce8f58f917d3555c04151cae3e0fc761291c632c0f13e618958cda614f7650efd18e1ffed06bc171530b5bde901becb2b021db47e541679bbb55f7337e1d205e1031eb2ef9a332e84bcbd9b5e27682159c86d3031ab01d741fedf1b05a4e1bc82da108855c8d833abb99821c8be81df68818e2aa094a3cde6f3d5e1bd8b2e86daba12aa2b572ffc81c65c3e498432edb00f1fe6fb04ec92a96b2c206a36a5623f8710c06cc20fdc661230f8b441ce4ebfa45a2890a4a43f2dbd498a9ef9d1f4e748ec81bca42a27aa8acd72406ca303050a32aa644f60e1c58a036ca2b0f0ca69092f6d08a40ee97ff700931b87039bdff71043a75b1578b33d98b391ae0dccbd46f5428cc80016412cbce9532b70454ab801c77072249412a2d49e8f608ab7480b1a9416714c825bd07a96641b865daeda71a5bf6b9e28bbf4a9042e79a6d6dd0c1d99dd3cabf4d580b6bc22999acbfb6f25a33e622104c13aa173e2191eb70dd0db82ae47d1ddbbdb24d3d4403bcaa64bcde88c86ce5bd535694b24f117e729abb3582e2166658f969a206b44aa37837c6efd7f094443c65f43b95826aa97ab1d3dc9aa3e56b31b1d2fe5e2eb44d2b5ccb1039118ac3917148fe4dffdc81daac74007ce71dd5e779be416de62b271bc4379e0a24c6c42e8e8c0213dee8588752d54b12b4da7be2e7a75c6b3e8ad9a92a4768d0611bc91ddd4aebda0cfb84226280ce6f621b5a83016b51bc9de2fe0413ce43ff967cf3680c9e1c359316ab207d93382df330b6a1fff25f01506952465fbdc1d36aeb0124b593b29619b712867c63c7e872d65d18b8834c505ff23688bda7b9d4e4969d6b69aae0b5ff14a152191c5f94a061aa3a7db71bc4ad9ce217a931a92b35e6faec8e00800f96b0efe6d42d1edf25573f12da245539f8fba9ab270cf738d585a144d9098e5d529f3e8662903de413ca9174b9bc6da0a70be23cce8b1dd7b0a1db109605f20d3ba5d72d4361e63cdbfe58ba1e19c0cbb0ab65090c8dc30265ac76a191707804756107d14e1ec41b6b8765286f99960fb601394bb1db089bd5ea19f3b98666e003affb6e9477a42f1c836efdb2355ce392849a777a6c4ac9e1eeb7bad57faa0f25eec2adf2d2d3b20e5ad1ce82ffdd9264e90a37a269f24841742479ccfadb38664c503191da287b16ca59d06b0ebb09e658cc0090665d8ca90b917c4a089b9474b9d76ba7d9deeb96f9a82ed10365d756da05f23bc9f81222b09e5d4b490977052f0bdd3afacd28b2f6730b3a6784eff8653fc8dcca5d17f7c34b05a30cf939ca10c52b714a5ab51b77b523917963a9f3374c7004b81588e06103615fb793dc267e8e3677d8fed75cb371"},"signature":"0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"}]`
	expectedPath := "/zond/v1/builder/validators"
	hc := &http.Client{
		Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			defer func() {
				require.NoError(t, r.Body.Close())
			}()
			require.NoError(t, err)
			require.Equal(t, expectedBody, string(body))
			require.Equal(t, expectedPath, r.URL.Path)
			require.Equal(t, http.MethodPost, r.Method)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBuffer(nil)),
				Request:    r.Clone(ctx),
			}, nil
		}),
	}
	c := &Client{
		hc:      hc,
		baseURL: &url.URL{Host: "localhost:3500", Scheme: "http"},
	}
	reg := &zond.SignedValidatorRegistrationV1{
		Message: &zond.ValidatorRegistrationV1{
			FeeRecipient: ezDecode(t, params.BeaconConfig().EthBurnAddressHex),
			GasLimit:     23,
			Timestamp:    42,
			Pubkey:       ezDecode(t, "0x94ef47878aea6c24a6aac5d43465cc361bbaf8bc8c9eba9abccda48977767f5604b8150337fd5ca7cf90bf8f63fca0e6fc0728a3071e5ccae2766a15679d2a57ddc95f6f51ff8bb831aaa937271eb80d599566ae1e73173aad708f68330bbd9c6555c0f9366763011f7aa4edebab101f99a4007c8ae1123a13e7c7cc19e2e7699b549bb770d3753bea49ec9e31104bae89fc38abe75e1f140267a2f492409d25f188aec4783afd2c8140f6a8c6850077536cb2760c65779d165b6f03f9b149210d8160f58803d31171be717baf887aa612c02f806bd9e3332ddc21c0e6e912d053d4d49c13d2de8a75266e6157610175d4897e58886aae12bf7b949e20351d80a4a994e7c70c9ba76a2472818343609061ffa393f6f270dc8b4aa806d5616e55e936f26cccd3e1006bf185196ae5457d224fca6555068bfb64a228e8b44b4221e85d2f1137224992f41a78301ff768527e953e50424a45d21e8877a16a915629f45aba1aca08769c561260d4d58bcf36af98c26e6b81365b91720f3155c1f9383d8c7ab295aabe6c9799f625d29da42bd4002fbd337ceaf991573678f6384da18541c4e3a3c9472620c19129bb41e2b5e71884b98a8ac319a0ec2be11948f4c02b0b824a2347e00945ec89f45f431dfb3605d8228ce69136867570bd0149f4fe2b53f19d1458e7d6f9688af7c5ec3021fbe61cb331a5a44c9f5e9a4a1192d5eeddf98f47e1c8379bf000ae6886cc3eb5442fa8586652550876a4ef31dd941eac082e9fb1ff26db706159ec7be0a5051b408fc955c55335db1f46c6e87113aabb03960f2f4fc986e4e583021b6b69e7c68b0d1093429630cf7f4a7e895ad45a41363c53d0ddce0804a8858092bc7a069852fff02773e0abd6c7cc7d3d34c0bebfb34e1c5e95ddb184c4a0fc77ed09fca96dc472ba0391214c489890ea410085d4c6ecc69f3facf0b1587372752a421641597563fedc9fc64452d7a7a0db9560ec1c8564a3180a5e623b65e1dd494f967556bde56f9bf58cf5e07a050258e4a90cc2700831a4113391877c65ebaadea8710e23ca9f7afb8c5bd90edf38211b32874b65bef5455159c1a1d17ac3da17819f8adcb254a62d7f4c362cc470f75fddaf6f6624f3a56b0187c4e21295534a832a0f2720c411d0685751fff095078d18bc854856f7e1abe14eb76e9e45fd1eca282804784d9c27fd15ab00bb6dbfd864c401f759d0f2da8ad8640b81066038b72c6cf26605f8388cdd67aeca21dda79d6dd01bc3d3ff5ee29f5e016ea681ba581940d0130685d42e9635c6ca27e1eb9fbf08f44879f4f479eefbde7b65476fbe379f771b0f116a2e6e5f65416b72f2c49452c40b2f108ab86ac7dbb8e252ed32946e5be280b512734c96db9511b21eeec3d0caeac51f8ca315ad7dd62efd1113e03932856d5eef73035a0ae24fef22a2c8aa1db28bc87702c34b2d2d722ddf0ad9d2eb4a6c16b85a7e22c49d115c676afeef8f66ec95ec805e4c1423df5dc2eeece55f107005e15b3ded7c5ff7b7d6fd8530049ede7b776bee594fc18e29a4ae177419bfcb2a0185f51ab35c389baf9b37742192e1c36c2ba7d5c46f683dfa8ca8103a824033c68d844d0ed55e560b96b8421162f57e2daaef5853bfe476903fcb42196e78ae2afe3d5da230de9628e2d268dfc50290b2a8ddae0ade58e2919e0f9be4b538220b885e20b28741cce7c6072aa27cf076197d8e046072045f4a4e20058e7ff428419ce7629b7f76dc0568d99586fc4095107b02228d4f2e9978abe68ef302747a67805d0158b88e94bdb73f1c6e1ca8918b311b0a5d101130e142d2085778d1546532c842f2673cb7a3774eb6d23fa901883aaa682aa7ed0195c3f899f6b485e09715a3791ac9102c177b37465344523fef3e9479ce39d358db0106b5b4cfe26415c5ebc2d00ab36af1171ca33f2c95ff16b63f91e10de4405aec7d6368afc20643616bf30505507f7af84a6f70a36c9f6644bf22bb141f88f15c01f99fbb0344b6db03c042cb8e80e6e38717c3e749cae3782b3d3529cbcfd68a04ade376a7f334ea471d21ebbc62035a57cced3f74a1612edb492a13aacc93d51348249451616f01bbd0e89f46fa53acc5490c7164ef8b2ac0a236b9da37f696db7d2e2dd51243966d2deeae418f2edc6f38fabe746ec5bc832db5af6856266c140e78ce15c2699778910c8e002290b52a68bd14c3be294154c7f448be0160d4adc856b307aa4d3a3ec82af0d25d951d25dc2027ed7861c9ef7d0228179a9fd38ba45f8721d6ce3dd42dce8f58f917d3555c04151cae3e0fc761291c632c0f13e618958cda614f7650efd18e1ffed06bc171530b5bde901becb2b021db47e541679bbb55f7337e1d205e1031eb2ef9a332e84bcbd9b5e27682159c86d3031ab01d741fedf1b05a4e1bc82da108855c8d833abb99821c8be81df68818e2aa094a3cde6f3d5e1bd8b2e86daba12aa2b572ffc81c65c3e498432edb00f1fe6fb04ec92a96b2c206a36a5623f8710c06cc20fdc661230f8b441ce4ebfa45a2890a4a43f2dbd498a9ef9d1f4e748ec81bca42a27aa8acd72406ca303050a32aa644f60e1c58a036ca2b0f0ca69092f6d08a40ee97ff700931b87039bdff71043a75b1578b33d98b391ae0dccbd46f5428cc80016412cbce9532b70454ab801c77072249412a2d49e8f608ab7480b1a9416714c825bd07a96641b865daeda71a5bf6b9e28bbf4a9042e79a6d6dd0c1d99dd3cabf4d580b6bc22999acbfb6f25a33e622104c13aa173e2191eb70dd0db82ae47d1ddbbdb24d3d4403bcaa64bcde88c86ce5bd535694b24f117e729abb3582e2166658f969a206b44aa37837c6efd7f094443c65f43b95826aa97ab1d3dc9aa3e56b31b1d2fe5e2eb44d2b5ccb1039118ac3917148fe4dffdc81daac74007ce71dd5e779be416de62b271bc4379e0a24c6c42e8e8c0213dee8588752d54b12b4da7be2e7a75c6b3e8ad9a92a4768d0611bc91ddd4aebda0cfb84226280ce6f621b5a83016b51bc9de2fe0413ce43ff967cf3680c9e1c359316ab207d93382df330b6a1fff25f01506952465fbdc1d36aeb0124b593b29619b712867c63c7e872d65d18b8834c505ff23688bda7b9d4e4969d6b69aae0b5ff14a152191c5f94a061aa3a7db71bc4ad9ce217a931a92b35e6faec8e00800f96b0efe6d42d1edf25573f12da245539f8fba9ab270cf738d585a144d9098e5d529f3e8662903de413ca9174b9bc6da0a70be23cce8b1dd7b0a1db109605f20d3ba5d72d4361e63cdbfe58ba1e19c0cbb0ab65090c8dc30265ac76a191707804756107d14e1ec41b6b8765286f99960fb601394bb1db089bd5ea19f3b98666e003affb6e9477a42f1c836efdb2355ce392849a777a6c4ac9e1eeb7bad57faa0f25eec2adf2d2d3b20e5ad1ce82ffdd9264e90a37a269f24841742479ccfadb38664c503191da287b16ca59d06b0ebb09e658cc0090665d8ca90b917c4a089b9474b9d76ba7d9deeb96f9a82ed10365d756da05f23bc9f81222b09e5d4b490977052f0bdd3afacd28b2f6730b3a6784eff8653fc8dcca5d17f7c34b05a30cf939ca10c52b714a5ab51b77b523917963a9f3374c7004b81588e06103615fb793dc267e8e3677d8fed75cb371"),
		},
		Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
	}
	require.NoError(t, c.RegisterValidator(ctx, []*zond.SignedValidatorRegistrationV1{reg}))
}

func TestClient_GetHeader(t *testing.T) {
	ctx := context.Background()
	expectedPath := "/zond/v1/builder/header/23/0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2/0x94ef47878aea6c24a6aac5d43465cc361bbaf8bc8c9eba9abccda48977767f5604b8150337fd5ca7cf90bf8f63fca0e6fc0728a3071e5ccae2766a15679d2a57ddc95f6f51ff8bb831aaa937271eb80d599566ae1e73173aad708f68330bbd9c6555c0f9366763011f7aa4edebab101f99a4007c8ae1123a13e7c7cc19e2e7699b549bb770d3753bea49ec9e31104bae89fc38abe75e1f140267a2f492409d25f188aec4783afd2c8140f6a8c6850077536cb2760c65779d165b6f03f9b149210d8160f58803d31171be717baf887aa612c02f806bd9e3332ddc21c0e6e912d053d4d49c13d2de8a75266e6157610175d4897e58886aae12bf7b949e20351d80a4a994e7c70c9ba76a2472818343609061ffa393f6f270dc8b4aa806d5616e55e936f26cccd3e1006bf185196ae5457d224fca6555068bfb64a228e8b44b4221e85d2f1137224992f41a78301ff768527e953e50424a45d21e8877a16a915629f45aba1aca08769c561260d4d58bcf36af98c26e6b81365b91720f3155c1f9383d8c7ab295aabe6c9799f625d29da42bd4002fbd337ceaf991573678f6384da18541c4e3a3c9472620c19129bb41e2b5e71884b98a8ac319a0ec2be11948f4c02b0b824a2347e00945ec89f45f431dfb3605d8228ce69136867570bd0149f4fe2b53f19d1458e7d6f9688af7c5ec3021fbe61cb331a5a44c9f5e9a4a1192d5eeddf98f47e1c8379bf000ae6886cc3eb5442fa8586652550876a4ef31dd941eac082e9fb1ff26db706159ec7be0a5051b408fc955c55335db1f46c6e87113aabb03960f2f4fc986e4e583021b6b69e7c68b0d1093429630cf7f4a7e895ad45a41363c53d0ddce0804a8858092bc7a069852fff02773e0abd6c7cc7d3d34c0bebfb34e1c5e95ddb184c4a0fc77ed09fca96dc472ba0391214c489890ea410085d4c6ecc69f3facf0b1587372752a421641597563fedc9fc64452d7a7a0db9560ec1c8564a3180a5e623b65e1dd494f967556bde56f9bf58cf5e07a050258e4a90cc2700831a4113391877c65ebaadea8710e23ca9f7afb8c5bd90edf38211b32874b65bef5455159c1a1d17ac3da17819f8adcb254a62d7f4c362cc470f75fddaf6f6624f3a56b0187c4e21295534a832a0f2720c411d0685751fff095078d18bc854856f7e1abe14eb76e9e45fd1eca282804784d9c27fd15ab00bb6dbfd864c401f759d0f2da8ad8640b81066038b72c6cf26605f8388cdd67aeca21dda79d6dd01bc3d3ff5ee29f5e016ea681ba581940d0130685d42e9635c6ca27e1eb9fbf08f44879f4f479eefbde7b65476fbe379f771b0f116a2e6e5f65416b72f2c49452c40b2f108ab86ac7dbb8e252ed32946e5be280b512734c96db9511b21eeec3d0caeac51f8ca315ad7dd62efd1113e03932856d5eef73035a0ae24fef22a2c8aa1db28bc87702c34b2d2d722ddf0ad9d2eb4a6c16b85a7e22c49d115c676afeef8f66ec95ec805e4c1423df5dc2eeece55f107005e15b3ded7c5ff7b7d6fd8530049ede7b776bee594fc18e29a4ae177419bfcb2a0185f51ab35c389baf9b37742192e1c36c2ba7d5c46f683dfa8ca8103a824033c68d844d0ed55e560b96b8421162f57e2daaef5853bfe476903fcb42196e78ae2afe3d5da230de9628e2d268dfc50290b2a8ddae0ade58e2919e0f9be4b538220b885e20b28741cce7c6072aa27cf076197d8e046072045f4a4e20058e7ff428419ce7629b7f76dc0568d99586fc4095107b02228d4f2e9978abe68ef302747a67805d0158b88e94bdb73f1c6e1ca8918b311b0a5d101130e142d2085778d1546532c842f2673cb7a3774eb6d23fa901883aaa682aa7ed0195c3f899f6b485e09715a3791ac9102c177b37465344523fef3e9479ce39d358db0106b5b4cfe26415c5ebc2d00ab36af1171ca33f2c95ff16b63f91e10de4405aec7d6368afc20643616bf30505507f7af84a6f70a36c9f6644bf22bb141f88f15c01f99fbb0344b6db03c042cb8e80e6e38717c3e749cae3782b3d3529cbcfd68a04ade376a7f334ea471d21ebbc62035a57cced3f74a1612edb492a13aacc93d51348249451616f01bbd0e89f46fa53acc5490c7164ef8b2ac0a236b9da37f696db7d2e2dd51243966d2deeae418f2edc6f38fabe746ec5bc832db5af6856266c140e78ce15c2699778910c8e002290b52a68bd14c3be294154c7f448be0160d4adc856b307aa4d3a3ec82af0d25d951d25dc2027ed7861c9ef7d0228179a9fd38ba45f8721d6ce3dd42dce8f58f917d3555c04151cae3e0fc761291c632c0f13e618958cda614f7650efd18e1ffed06bc171530b5bde901becb2b021db47e541679bbb55f7337e1d205e1031eb2ef9a332e84bcbd9b5e27682159c86d3031ab01d741fedf1b05a4e1bc82da108855c8d833abb99821c8be81df68818e2aa094a3cde6f3d5e1bd8b2e86daba12aa2b572ffc81c65c3e498432edb00f1fe6fb04ec92a96b2c206a36a5623f8710c06cc20fdc661230f8b441ce4ebfa45a2890a4a43f2dbd498a9ef9d1f4e748ec81bca42a27aa8acd72406ca303050a32aa644f60e1c58a036ca2b0f0ca69092f6d08a40ee97ff700931b87039bdff71043a75b1578b33d98b391ae0dccbd46f5428cc80016412cbce9532b70454ab801c77072249412a2d49e8f608ab7480b1a9416714c825bd07a96641b865daeda71a5bf6b9e28bbf4a9042e79a6d6dd0c1d99dd3cabf4d580b6bc22999acbfb6f25a33e622104c13aa173e2191eb70dd0db82ae47d1ddbbdb24d3d4403bcaa64bcde88c86ce5bd535694b24f117e729abb3582e2166658f969a206b44aa37837c6efd7f094443c65f43b95826aa97ab1d3dc9aa3e56b31b1d2fe5e2eb44d2b5ccb1039118ac3917148fe4dffdc81daac74007ce71dd5e779be416de62b271bc4379e0a24c6c42e8e8c0213dee8588752d54b12b4da7be2e7a75c6b3e8ad9a92a4768d0611bc91ddd4aebda0cfb84226280ce6f621b5a83016b51bc9de2fe0413ce43ff967cf3680c9e1c359316ab207d93382df330b6a1fff25f01506952465fbdc1d36aeb0124b593b29619b712867c63c7e872d65d18b8834c505ff23688bda7b9d4e4969d6b69aae0b5ff14a152191c5f94a061aa3a7db71bc4ad9ce217a931a92b35e6faec8e00800f96b0efe6d42d1edf25573f12da245539f8fba9ab270cf738d585a144d9098e5d529f3e8662903de413ca9174b9bc6da0a70be23cce8b1dd7b0a1db109605f20d3ba5d72d4361e63cdbfe58ba1e19c0cbb0ab65090c8dc30265ac76a191707804756107d14e1ec41b6b8765286f99960fb601394bb1db089bd5ea19f3b98666e003affb6e9477a42f1c836efdb2355ce392849a777a6c4ac9e1eeb7bad57faa0f25eec2adf2d2d3b20e5ad1ce82ffdd9264e90a37a269f24841742479ccfadb38664c503191da287b16ca59d06b0ebb09e658cc0090665d8ca90b917c4a089b9474b9d76ba7d9deeb96f9a82ed10365d756da05f23bc9f81222b09e5d4b490977052f0bdd3afacd28b2f6730b3a6784eff8653fc8dcca5d17f7c34b05a30cf939ca10c52b714a5ab51b77b523917963a9f3374c7004b81588e06103615fb793dc267e8e3677d8fed75cb371"
	var slot types.Slot = 23
	parentHash := ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2")
	pubkey := ezDecode(t, "0x94ef47878aea6c24a6aac5d43465cc361bbaf8bc8c9eba9abccda48977767f5604b8150337fd5ca7cf90bf8f63fca0e6fc0728a3071e5ccae2766a15679d2a57ddc95f6f51ff8bb831aaa937271eb80d599566ae1e73173aad708f68330bbd9c6555c0f9366763011f7aa4edebab101f99a4007c8ae1123a13e7c7cc19e2e7699b549bb770d3753bea49ec9e31104bae89fc38abe75e1f140267a2f492409d25f188aec4783afd2c8140f6a8c6850077536cb2760c65779d165b6f03f9b149210d8160f58803d31171be717baf887aa612c02f806bd9e3332ddc21c0e6e912d053d4d49c13d2de8a75266e6157610175d4897e58886aae12bf7b949e20351d80a4a994e7c70c9ba76a2472818343609061ffa393f6f270dc8b4aa806d5616e55e936f26cccd3e1006bf185196ae5457d224fca6555068bfb64a228e8b44b4221e85d2f1137224992f41a78301ff768527e953e50424a45d21e8877a16a915629f45aba1aca08769c561260d4d58bcf36af98c26e6b81365b91720f3155c1f9383d8c7ab295aabe6c9799f625d29da42bd4002fbd337ceaf991573678f6384da18541c4e3a3c9472620c19129bb41e2b5e71884b98a8ac319a0ec2be11948f4c02b0b824a2347e00945ec89f45f431dfb3605d8228ce69136867570bd0149f4fe2b53f19d1458e7d6f9688af7c5ec3021fbe61cb331a5a44c9f5e9a4a1192d5eeddf98f47e1c8379bf000ae6886cc3eb5442fa8586652550876a4ef31dd941eac082e9fb1ff26db706159ec7be0a5051b408fc955c55335db1f46c6e87113aabb03960f2f4fc986e4e583021b6b69e7c68b0d1093429630cf7f4a7e895ad45a41363c53d0ddce0804a8858092bc7a069852fff02773e0abd6c7cc7d3d34c0bebfb34e1c5e95ddb184c4a0fc77ed09fca96dc472ba0391214c489890ea410085d4c6ecc69f3facf0b1587372752a421641597563fedc9fc64452d7a7a0db9560ec1c8564a3180a5e623b65e1dd494f967556bde56f9bf58cf5e07a050258e4a90cc2700831a4113391877c65ebaadea8710e23ca9f7afb8c5bd90edf38211b32874b65bef5455159c1a1d17ac3da17819f8adcb254a62d7f4c362cc470f75fddaf6f6624f3a56b0187c4e21295534a832a0f2720c411d0685751fff095078d18bc854856f7e1abe14eb76e9e45fd1eca282804784d9c27fd15ab00bb6dbfd864c401f759d0f2da8ad8640b81066038b72c6cf26605f8388cdd67aeca21dda79d6dd01bc3d3ff5ee29f5e016ea681ba581940d0130685d42e9635c6ca27e1eb9fbf08f44879f4f479eefbde7b65476fbe379f771b0f116a2e6e5f65416b72f2c49452c40b2f108ab86ac7dbb8e252ed32946e5be280b512734c96db9511b21eeec3d0caeac51f8ca315ad7dd62efd1113e03932856d5eef73035a0ae24fef22a2c8aa1db28bc87702c34b2d2d722ddf0ad9d2eb4a6c16b85a7e22c49d115c676afeef8f66ec95ec805e4c1423df5dc2eeece55f107005e15b3ded7c5ff7b7d6fd8530049ede7b776bee594fc18e29a4ae177419bfcb2a0185f51ab35c389baf9b37742192e1c36c2ba7d5c46f683dfa8ca8103a824033c68d844d0ed55e560b96b8421162f57e2daaef5853bfe476903fcb42196e78ae2afe3d5da230de9628e2d268dfc50290b2a8ddae0ade58e2919e0f9be4b538220b885e20b28741cce7c6072aa27cf076197d8e046072045f4a4e20058e7ff428419ce7629b7f76dc0568d99586fc4095107b02228d4f2e9978abe68ef302747a67805d0158b88e94bdb73f1c6e1ca8918b311b0a5d101130e142d2085778d1546532c842f2673cb7a3774eb6d23fa901883aaa682aa7ed0195c3f899f6b485e09715a3791ac9102c177b37465344523fef3e9479ce39d358db0106b5b4cfe26415c5ebc2d00ab36af1171ca33f2c95ff16b63f91e10de4405aec7d6368afc20643616bf30505507f7af84a6f70a36c9f6644bf22bb141f88f15c01f99fbb0344b6db03c042cb8e80e6e38717c3e749cae3782b3d3529cbcfd68a04ade376a7f334ea471d21ebbc62035a57cced3f74a1612edb492a13aacc93d51348249451616f01bbd0e89f46fa53acc5490c7164ef8b2ac0a236b9da37f696db7d2e2dd51243966d2deeae418f2edc6f38fabe746ec5bc832db5af6856266c140e78ce15c2699778910c8e002290b52a68bd14c3be294154c7f448be0160d4adc856b307aa4d3a3ec82af0d25d951d25dc2027ed7861c9ef7d0228179a9fd38ba45f8721d6ce3dd42dce8f58f917d3555c04151cae3e0fc761291c632c0f13e618958cda614f7650efd18e1ffed06bc171530b5bde901becb2b021db47e541679bbb55f7337e1d205e1031eb2ef9a332e84bcbd9b5e27682159c86d3031ab01d741fedf1b05a4e1bc82da108855c8d833abb99821c8be81df68818e2aa094a3cde6f3d5e1bd8b2e86daba12aa2b572ffc81c65c3e498432edb00f1fe6fb04ec92a96b2c206a36a5623f8710c06cc20fdc661230f8b441ce4ebfa45a2890a4a43f2dbd498a9ef9d1f4e748ec81bca42a27aa8acd72406ca303050a32aa644f60e1c58a036ca2b0f0ca69092f6d08a40ee97ff700931b87039bdff71043a75b1578b33d98b391ae0dccbd46f5428cc80016412cbce9532b70454ab801c77072249412a2d49e8f608ab7480b1a9416714c825bd07a96641b865daeda71a5bf6b9e28bbf4a9042e79a6d6dd0c1d99dd3cabf4d580b6bc22999acbfb6f25a33e622104c13aa173e2191eb70dd0db82ae47d1ddbbdb24d3d4403bcaa64bcde88c86ce5bd535694b24f117e729abb3582e2166658f969a206b44aa37837c6efd7f094443c65f43b95826aa97ab1d3dc9aa3e56b31b1d2fe5e2eb44d2b5ccb1039118ac3917148fe4dffdc81daac74007ce71dd5e779be416de62b271bc4379e0a24c6c42e8e8c0213dee8588752d54b12b4da7be2e7a75c6b3e8ad9a92a4768d0611bc91ddd4aebda0cfb84226280ce6f621b5a83016b51bc9de2fe0413ce43ff967cf3680c9e1c359316ab207d93382df330b6a1fff25f01506952465fbdc1d36aeb0124b593b29619b712867c63c7e872d65d18b8834c505ff23688bda7b9d4e4969d6b69aae0b5ff14a152191c5f94a061aa3a7db71bc4ad9ce217a931a92b35e6faec8e00800f96b0efe6d42d1edf25573f12da245539f8fba9ab270cf738d585a144d9098e5d529f3e8662903de413ca9174b9bc6da0a70be23cce8b1dd7b0a1db109605f20d3ba5d72d4361e63cdbfe58ba1e19c0cbb0ab65090c8dc30265ac76a191707804756107d14e1ec41b6b8765286f99960fb601394bb1db089bd5ea19f3b98666e003affb6e9477a42f1c836efdb2355ce392849a777a6c4ac9e1eeb7bad57faa0f25eec2adf2d2d3b20e5ad1ce82ffdd9264e90a37a269f24841742479ccfadb38664c503191da287b16ca59d06b0ebb09e658cc0090665d8ca90b917c4a089b9474b9d76ba7d9deeb96f9a82ed10365d756da05f23bc9f81222b09e5d4b490977052f0bdd3afacd28b2f6730b3a6784eff8653fc8dcca5d17f7c34b05a30cf939ca10c52b714a5ab51b77b523917963a9f3374c7004b81588e06103615fb793dc267e8e3677d8fed75cb371")

	t.Run("server error", func(t *testing.T) {
		hc := &http.Client{
			Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, expectedPath, r.URL.Path)
				message := ErrorMessage{
					Code:    500,
					Message: "Internal server error",
				}
				resp, err := json.Marshal(message)
				require.NoError(t, err)
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(bytes.NewBuffer(resp)),
					Request:    r.Clone(ctx),
				}, nil
			}),
		}
		c := &Client{
			hc:      hc,
			baseURL: &url.URL{Host: "localhost:3500", Scheme: "http"},
		}

		_, err := c.GetHeader(ctx, slot, bytesutil.ToBytes32(parentHash), bytesutil.ToBytes2592(pubkey))
		require.ErrorIs(t, err, ErrNotOK)
	})
	t.Run("header not available", func(t *testing.T) {
		hc := &http.Client{
			Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, expectedPath, r.URL.Path)
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       io.NopCloser(bytes.NewBuffer([]byte("No header is available."))),
					Request:    r.Clone(ctx),
				}, nil
			}),
		}
		c := &Client{
			hc:      hc,
			baseURL: &url.URL{Host: "localhost:3500", Scheme: "http"},
		}
		_, err := c.GetHeader(ctx, slot, bytesutil.ToBytes32(parentHash), bytesutil.ToBytes2592(pubkey))
		require.ErrorIs(t, err, ErrNoContent)
	})
	t.Run("bellatrix", func(t *testing.T) {
		hc := &http.Client{
			Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, expectedPath, r.URL.Path)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(testExampleHeaderResponse)),
					Request:    r.Clone(ctx),
				}, nil
			}),
		}
		c := &Client{
			hc:      hc,
			baseURL: &url.URL{Host: "localhost:3500", Scheme: "http"},
		}
		h, err := c.GetHeader(ctx, slot, bytesutil.ToBytes32(parentHash), bytesutil.ToBytes2592(pubkey))
		require.NoError(t, err)
		expectedSig := ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505")
		require.Equal(t, true, bytes.Equal(expectedSig, h.Signature()))
		expectedTxRoot := ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2")
		bid, err := h.Message()
		require.NoError(t, err)
		bidHeader, err := bid.Header()
		require.NoError(t, err)
		withdrawalsRoot, err := bidHeader.TransactionsRoot()
		require.NoError(t, err)
		require.Equal(t, true, bytes.Equal(expectedTxRoot, withdrawalsRoot))
		require.Equal(t, uint64(1), bidHeader.GasUsed())
		value, err := stringToUint256("652312848583266388373324160190187140051835877600158453279131187530910662656")
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%#x", value.SSZBytes()), fmt.Sprintf("%#x", bid.Value()))
		bidValue := bytesutil.ReverseByteOrder(bid.Value())
		require.DeepEqual(t, bidValue, value.Bytes())
		require.DeepEqual(t, big.NewInt(0).SetBytes(bidValue), value.Int)
	})
	t.Run("capella", func(t *testing.T) {
		hc := &http.Client{
			Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, expectedPath, r.URL.Path)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(testExampleHeaderResponseCapella)),
					Request:    r.Clone(ctx),
				}, nil
			}),
		}
		c := &Client{
			hc:      hc,
			baseURL: &url.URL{Host: "localhost:3500", Scheme: "http"},
		}
		h, err := c.GetHeader(ctx, slot, bytesutil.ToBytes32(parentHash), bytesutil.ToBytes2592(pubkey))
		require.NoError(t, err)
		expectedWithdrawalsRoot := ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2")
		bid, err := h.Message()
		require.NoError(t, err)
		bidHeader, err := bid.Header()
		require.NoError(t, err)
		withdrawalsRoot, err := bidHeader.WithdrawalsRoot()
		require.NoError(t, err)
		require.Equal(t, true, bytes.Equal(expectedWithdrawalsRoot, withdrawalsRoot))
		value, err := stringToUint256("652312848583266388373324160190187140051835877600158453279131187530910662656")
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%#x", value.SSZBytes()), fmt.Sprintf("%#x", bid.Value()))
		bidValue := bytesutil.ReverseByteOrder(bid.Value())
		require.DeepEqual(t, bidValue, value.Bytes())
		require.DeepEqual(t, big.NewInt(0).SetBytes(bidValue), value.Int)
	})
	t.Run("unsupported version", func(t *testing.T) {
		hc := &http.Client{
			Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, expectedPath, r.URL.Path)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(testExampleHeaderResponseUnknownVersion)),
					Request:    r.Clone(ctx),
				}, nil
			}),
		}
		c := &Client{
			hc:      hc,
			baseURL: &url.URL{Host: "localhost:3500", Scheme: "http"},
		}
		_, err := c.GetHeader(ctx, slot, bytesutil.ToBytes32(parentHash), bytesutil.ToBytes2592(pubkey))
		require.ErrorContains(t, "unsupported header version", err)
	})
}

func TestSubmitBlindedBlock(t *testing.T) {
	ctx := context.Background()

	t.Run("bellatrix", func(t *testing.T) {
		hc := &http.Client{
			Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, postBlindedBeaconBlockPath, r.URL.Path)
				require.Equal(t, "bellatrix", r.Header.Get("Eth-Consensus-Version"))
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(testExampleExecutionPayload)),
					Request:    r.Clone(ctx),
				}, nil
			}),
		}
		c := &Client{
			hc:      hc,
			baseURL: &url.URL{Host: "localhost:3500", Scheme: "http"},
		}
		sbbb, err := blocks.NewSignedBeaconBlock(testSignedBlindedBeaconBlockBellatrix(t))
		require.NoError(t, err)
		ep, err := c.SubmitBlindedBlock(ctx, sbbb)
		require.NoError(t, err)
		require.Equal(t, true, bytes.Equal(ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"), ep.ParentHash()))
		bfpg, err := stringToUint256("452312848583266388373324160190187140051835877600158453279131187530910662656")
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%#x", bfpg.SSZBytes()), fmt.Sprintf("%#x", ep.BaseFeePerGas()))
		require.Equal(t, uint64(1), ep.GasLimit())
	})
	t.Run("capella", func(t *testing.T) {
		hc := &http.Client{
			Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, postBlindedBeaconBlockPath, r.URL.Path)
				require.Equal(t, "capella", r.Header.Get("Eth-Consensus-Version"))
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(testExampleExecutionPayloadCapella)),
					Request:    r.Clone(ctx),
				}, nil
			}),
		}
		c := &Client{
			hc:      hc,
			baseURL: &url.URL{Host: "localhost:3500", Scheme: "http"},
		}
		sbb, err := blocks.NewSignedBeaconBlock(testSignedBlindedBeaconBlockCapella(t))
		require.NoError(t, err)
		ep, err := c.SubmitBlindedBlock(ctx, sbb)
		require.NoError(t, err)
		withdrawals, err := ep.Withdrawals()
		require.NoError(t, err)
		require.Equal(t, 1, len(withdrawals))
		assert.Equal(t, uint64(1), withdrawals[0].Index)
		assert.Equal(t, types.ValidatorIndex(1), withdrawals[0].ValidatorIndex)
		assert.DeepEqual(t, ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943"), withdrawals[0].Address)
		assert.Equal(t, uint64(1), withdrawals[0].Amount)
	})
	t.Run("mismatched versions, expected bellatrix got capella", func(t *testing.T) {
		hc := &http.Client{
			Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, postBlindedBeaconBlockPath, r.URL.Path)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(testExampleExecutionPayloadCapella)), // send a Capella payload
					Request:    r.Clone(ctx),
				}, nil
			}),
		}
		c := &Client{
			hc:      hc,
			baseURL: &url.URL{Host: "localhost:3500", Scheme: "http"},
		}
		sbbb, err := blocks.NewSignedBeaconBlock(testSignedBlindedBeaconBlockBellatrix(t))
		require.NoError(t, err)
		_, err = c.SubmitBlindedBlock(ctx, sbbb)
		require.ErrorContains(t, "not a bellatrix payload", err)
	})
	t.Run("not blinded", func(t *testing.T) {
		sbb, err := blocks.NewSignedBeaconBlock(&zond.SignedBeaconBlockBellatrix{Block: &zond.BeaconBlockBellatrix{Body: &zond.BeaconBlockBodyBellatrix{}}})
		require.NoError(t, err)
		_, err = (&Client{}).SubmitBlindedBlock(ctx, sbb)
		require.ErrorIs(t, err, errNotBlinded)
	})
}

func testSignedBlindedBeaconBlockBellatrix(t *testing.T) *zond.SignedBlindedBeaconBlockBellatrix {
	return &zond.SignedBlindedBeaconBlockBellatrix{
		Block: &zond.BlindedBeaconBlockBellatrix{
			Slot:          1,
			ProposerIndex: 1,
			ParentRoot:    ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
			StateRoot:     ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
			Body: &zond.BlindedBeaconBlockBodyBellatrix{
				RandaoReveal: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
				Eth1Data: &zond.Eth1Data{
					DepositRoot:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					DepositCount: 1,
					BlockHash:    ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
				},
				Graffiti: ezDecode(t, "0xdeadbeefc0ffee"),
				ProposerSlashings: []*zond.ProposerSlashing{
					{
						Header_1: &zond.SignedBeaconBlockHeader{
							Header: &zond.BeaconBlockHeader{
								Slot:          1,
								ProposerIndex: 1,
								ParentRoot:    ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								StateRoot:     ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								BodyRoot:      ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							},
							Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
						},
						Header_2: &zond.SignedBeaconBlockHeader{
							Header: &zond.BeaconBlockHeader{
								Slot:          1,
								ProposerIndex: 1,
								ParentRoot:    ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								StateRoot:     ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								BodyRoot:      ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							},
							Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
						},
					},
				},
				AttesterSlashings: []*zond.AttesterSlashing{
					{
						Attestation_1: &zond.IndexedAttestation{
							AttestingIndices: []uint64{1},
							Data: &zond.AttestationData{
								Slot:            1,
								CommitteeIndex:  1,
								BeaconBlockRoot: ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								Source: &zond.Checkpoint{
									Epoch: 1,
									Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								},
								Target: &zond.Checkpoint{
									Epoch: 1,
									Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								},
							},
							Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
						},
						Attestation_2: &zond.IndexedAttestation{
							AttestingIndices: []uint64{1},
							Data: &zond.AttestationData{
								Slot:            1,
								CommitteeIndex:  1,
								BeaconBlockRoot: ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								Source: &zond.Checkpoint{
									Epoch: 1,
									Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								},
								Target: &zond.Checkpoint{
									Epoch: 1,
									Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								},
							},
							Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
						},
					},
				},
				Attestations: []*zond.Attestation{
					{
						AggregationBits: bitfield.Bitlist{0x01},
						Data: &zond.AttestationData{
							Slot:            1,
							CommitteeIndex:  1,
							BeaconBlockRoot: ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							Source: &zond.Checkpoint{
								Epoch: 1,
								Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							},
							Target: &zond.Checkpoint{
								Epoch: 1,
								Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							},
						},
						Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
					},
				},
				Deposits: []*zond.Deposit{
					{
						Proof: [][]byte{ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2")},
						Data: &zond.Deposit_Data{
							PublicKey:             ezDecode(t, "0x94ef47878aea6c24a6aac5d43465cc361bbaf8bc8c9eba9abccda48977767f5604b8150337fd5ca7cf90bf8f63fca0e6fc0728a3071e5ccae2766a15679d2a57ddc95f6f51ff8bb831aaa937271eb80d599566ae1e73173aad708f68330bbd9c6555c0f9366763011f7aa4edebab101f99a4007c8ae1123a13e7c7cc19e2e7699b549bb770d3753bea49ec9e31104bae89fc38abe75e1f140267a2f492409d25f188aec4783afd2c8140f6a8c6850077536cb2760c65779d165b6f03f9b149210d8160f58803d31171be717baf887aa612c02f806bd9e3332ddc21c0e6e912d053d4d49c13d2de8a75266e6157610175d4897e58886aae12bf7b949e20351d80a4a994e7c70c9ba76a2472818343609061ffa393f6f270dc8b4aa806d5616e55e936f26cccd3e1006bf185196ae5457d224fca6555068bfb64a228e8b44b4221e85d2f1137224992f41a78301ff768527e953e50424a45d21e8877a16a915629f45aba1aca08769c561260d4d58bcf36af98c26e6b81365b91720f3155c1f9383d8c7ab295aabe6c9799f625d29da42bd4002fbd337ceaf991573678f6384da18541c4e3a3c9472620c19129bb41e2b5e71884b98a8ac319a0ec2be11948f4c02b0b824a2347e00945ec89f45f431dfb3605d8228ce69136867570bd0149f4fe2b53f19d1458e7d6f9688af7c5ec3021fbe61cb331a5a44c9f5e9a4a1192d5eeddf98f47e1c8379bf000ae6886cc3eb5442fa8586652550876a4ef31dd941eac082e9fb1ff26db706159ec7be0a5051b408fc955c55335db1f46c6e87113aabb03960f2f4fc986e4e583021b6b69e7c68b0d1093429630cf7f4a7e895ad45a41363c53d0ddce0804a8858092bc7a069852fff02773e0abd6c7cc7d3d34c0bebfb34e1c5e95ddb184c4a0fc77ed09fca96dc472ba0391214c489890ea410085d4c6ecc69f3facf0b1587372752a421641597563fedc9fc64452d7a7a0db9560ec1c8564a3180a5e623b65e1dd494f967556bde56f9bf58cf5e07a050258e4a90cc2700831a4113391877c65ebaadea8710e23ca9f7afb8c5bd90edf38211b32874b65bef5455159c1a1d17ac3da17819f8adcb254a62d7f4c362cc470f75fddaf6f6624f3a56b0187c4e21295534a832a0f2720c411d0685751fff095078d18bc854856f7e1abe14eb76e9e45fd1eca282804784d9c27fd15ab00bb6dbfd864c401f759d0f2da8ad8640b81066038b72c6cf26605f8388cdd67aeca21dda79d6dd01bc3d3ff5ee29f5e016ea681ba581940d0130685d42e9635c6ca27e1eb9fbf08f44879f4f479eefbde7b65476fbe379f771b0f116a2e6e5f65416b72f2c49452c40b2f108ab86ac7dbb8e252ed32946e5be280b512734c96db9511b21eeec3d0caeac51f8ca315ad7dd62efd1113e03932856d5eef73035a0ae24fef22a2c8aa1db28bc87702c34b2d2d722ddf0ad9d2eb4a6c16b85a7e22c49d115c676afeef8f66ec95ec805e4c1423df5dc2eeece55f107005e15b3ded7c5ff7b7d6fd8530049ede7b776bee594fc18e29a4ae177419bfcb2a0185f51ab35c389baf9b37742192e1c36c2ba7d5c46f683dfa8ca8103a824033c68d844d0ed55e560b96b8421162f57e2daaef5853bfe476903fcb42196e78ae2afe3d5da230de9628e2d268dfc50290b2a8ddae0ade58e2919e0f9be4b538220b885e20b28741cce7c6072aa27cf076197d8e046072045f4a4e20058e7ff428419ce7629b7f76dc0568d99586fc4095107b02228d4f2e9978abe68ef302747a67805d0158b88e94bdb73f1c6e1ca8918b311b0a5d101130e142d2085778d1546532c842f2673cb7a3774eb6d23fa901883aaa682aa7ed0195c3f899f6b485e09715a3791ac9102c177b37465344523fef3e9479ce39d358db0106b5b4cfe26415c5ebc2d00ab36af1171ca33f2c95ff16b63f91e10de4405aec7d6368afc20643616bf30505507f7af84a6f70a36c9f6644bf22bb141f88f15c01f99fbb0344b6db03c042cb8e80e6e38717c3e749cae3782b3d3529cbcfd68a04ade376a7f334ea471d21ebbc62035a57cced3f74a1612edb492a13aacc93d51348249451616f01bbd0e89f46fa53acc5490c7164ef8b2ac0a236b9da37f696db7d2e2dd51243966d2deeae418f2edc6f38fabe746ec5bc832db5af6856266c140e78ce15c2699778910c8e002290b52a68bd14c3be294154c7f448be0160d4adc856b307aa4d3a3ec82af0d25d951d25dc2027ed7861c9ef7d0228179a9fd38ba45f8721d6ce3dd42dce8f58f917d3555c04151cae3e0fc761291c632c0f13e618958cda614f7650efd18e1ffed06bc171530b5bde901becb2b021db47e541679bbb55f7337e1d205e1031eb2ef9a332e84bcbd9b5e27682159c86d3031ab01d741fedf1b05a4e1bc82da108855c8d833abb99821c8be81df68818e2aa094a3cde6f3d5e1bd8b2e86daba12aa2b572ffc81c65c3e498432edb00f1fe6fb04ec92a96b2c206a36a5623f8710c06cc20fdc661230f8b441ce4ebfa45a2890a4a43f2dbd498a9ef9d1f4e748ec81bca42a27aa8acd72406ca303050a32aa644f60e1c58a036ca2b0f0ca69092f6d08a40ee97ff700931b87039bdff71043a75b1578b33d98b391ae0dccbd46f5428cc80016412cbce9532b70454ab801c77072249412a2d49e8f608ab7480b1a9416714c825bd07a96641b865daeda71a5bf6b9e28bbf4a9042e79a6d6dd0c1d99dd3cabf4d580b6bc22999acbfb6f25a33e622104c13aa173e2191eb70dd0db82ae47d1ddbbdb24d3d4403bcaa64bcde88c86ce5bd535694b24f117e729abb3582e2166658f969a206b44aa37837c6efd7f094443c65f43b95826aa97ab1d3dc9aa3e56b31b1d2fe5e2eb44d2b5ccb1039118ac3917148fe4dffdc81daac74007ce71dd5e779be416de62b271bc4379e0a24c6c42e8e8c0213dee8588752d54b12b4da7be2e7a75c6b3e8ad9a92a4768d0611bc91ddd4aebda0cfb84226280ce6f621b5a83016b51bc9de2fe0413ce43ff967cf3680c9e1c359316ab207d93382df330b6a1fff25f01506952465fbdc1d36aeb0124b593b29619b712867c63c7e872d65d18b8834c505ff23688bda7b9d4e4969d6b69aae0b5ff14a152191c5f94a061aa3a7db71bc4ad9ce217a931a92b35e6faec8e00800f96b0efe6d42d1edf25573f12da245539f8fba9ab270cf738d585a144d9098e5d529f3e8662903de413ca9174b9bc6da0a70be23cce8b1dd7b0a1db109605f20d3ba5d72d4361e63cdbfe58ba1e19c0cbb0ab65090c8dc30265ac76a191707804756107d14e1ec41b6b8765286f99960fb601394bb1db089bd5ea19f3b98666e003affb6e9477a42f1c836efdb2355ce392849a777a6c4ac9e1eeb7bad57faa0f25eec2adf2d2d3b20e5ad1ce82ffdd9264e90a37a269f24841742479ccfadb38664c503191da287b16ca59d06b0ebb09e658cc0090665d8ca90b917c4a089b9474b9d76ba7d9deeb96f9a82ed10365d756da05f23bc9f81222b09e5d4b490977052f0bdd3afacd28b2f6730b3a6784eff8653fc8dcca5d17f7c34b05a30cf939ca10c52b714a5ab51b77b523917963a9f3374c7004b81588e06103615fb793dc267e8e3677d8fed75cb371"),
							WithdrawalCredentials: ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							Amount:                1,
							Signature:             ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
						},
					},
				},
				VoluntaryExits: []*zond.SignedVoluntaryExit{
					{
						Exit: &zond.VoluntaryExit{
							Epoch:          1,
							ValidatorIndex: 1,
						},
						Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
					},
				},
				SyncAggregate: &zond.SyncAggregate{
					SyncCommitteeSignature: make([]byte, 48),
					SyncCommitteeBits:      bitfield.Bitvector512{0x01},
				},
				ExecutionPayloadHeader: &v1.ExecutionPayloadHeader{
					ParentHash:       ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					FeeRecipient:     ezDecode(t, "0xabcf8e0d4e9587369b2301d0790347320302cc09"),
					StateRoot:        ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					ReceiptsRoot:     ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					LogsBloom:        ezDecode(t, "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
					PrevRandao:       ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					BlockNumber:      1,
					GasLimit:         1,
					GasUsed:          1,
					Timestamp:        1,
					ExtraData:        ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					BaseFeePerGas:    []byte(strconv.FormatUint(1, 10)),
					BlockHash:        ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					TransactionsRoot: ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
				},
			},
		},
		Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
	}
}

func testSignedBlindedBeaconBlockCapella(t *testing.T) *zond.SignedBlindedBeaconBlockCapella {
	return &zond.SignedBlindedBeaconBlockCapella{
		Block: &zond.BlindedBeaconBlockCapella{
			Slot:          1,
			ProposerIndex: 1,
			ParentRoot:    ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
			StateRoot:     ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
			Body: &zond.BlindedBeaconBlockBodyCapella{
				RandaoReveal: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
				Eth1Data: &zond.Eth1Data{
					DepositRoot:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					DepositCount: 1,
					BlockHash:    ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
				},
				Graffiti: ezDecode(t, "0xdeadbeefc0ffee"),
				ProposerSlashings: []*zond.ProposerSlashing{
					{
						Header_1: &zond.SignedBeaconBlockHeader{
							Header: &zond.BeaconBlockHeader{
								Slot:          1,
								ProposerIndex: 1,
								ParentRoot:    ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								StateRoot:     ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								BodyRoot:      ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							},
							Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
						},
						Header_2: &zond.SignedBeaconBlockHeader{
							Header: &zond.BeaconBlockHeader{
								Slot:          1,
								ProposerIndex: 1,
								ParentRoot:    ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								StateRoot:     ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								BodyRoot:      ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							},
							Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
						},
					},
				},
				AttesterSlashings: []*zond.AttesterSlashing{
					{
						Attestation_1: &zond.IndexedAttestation{
							AttestingIndices: []uint64{1},
							Data: &zond.AttestationData{
								Slot:            1,
								CommitteeIndex:  1,
								BeaconBlockRoot: ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								Source: &zond.Checkpoint{
									Epoch: 1,
									Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								},
								Target: &zond.Checkpoint{
									Epoch: 1,
									Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								},
							},
							Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
						},
						Attestation_2: &zond.IndexedAttestation{
							AttestingIndices: []uint64{1},
							Data: &zond.AttestationData{
								Slot:            1,
								CommitteeIndex:  1,
								BeaconBlockRoot: ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								Source: &zond.Checkpoint{
									Epoch: 1,
									Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								},
								Target: &zond.Checkpoint{
									Epoch: 1,
									Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
								},
							},
							Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
						},
					},
				},
				Attestations: []*zond.Attestation{
					{
						AggregationBits: bitfield.Bitlist{0x01},
						Data: &zond.AttestationData{
							Slot:            1,
							CommitteeIndex:  1,
							BeaconBlockRoot: ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							Source: &zond.Checkpoint{
								Epoch: 1,
								Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							},
							Target: &zond.Checkpoint{
								Epoch: 1,
								Root:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							},
						},
						Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
					},
				},
				Deposits: []*zond.Deposit{
					{
						Proof: [][]byte{ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2")},
						Data: &zond.Deposit_Data{
							PublicKey:             ezDecode(t, "0x94ef47878aea6c24a6aac5d43465cc361bbaf8bc8c9eba9abccda48977767f5604b8150337fd5ca7cf90bf8f63fca0e6fc0728a3071e5ccae2766a15679d2a57ddc95f6f51ff8bb831aaa937271eb80d599566ae1e73173aad708f68330bbd9c6555c0f9366763011f7aa4edebab101f99a4007c8ae1123a13e7c7cc19e2e7699b549bb770d3753bea49ec9e31104bae89fc38abe75e1f140267a2f492409d25f188aec4783afd2c8140f6a8c6850077536cb2760c65779d165b6f03f9b149210d8160f58803d31171be717baf887aa612c02f806bd9e3332ddc21c0e6e912d053d4d49c13d2de8a75266e6157610175d4897e58886aae12bf7b949e20351d80a4a994e7c70c9ba76a2472818343609061ffa393f6f270dc8b4aa806d5616e55e936f26cccd3e1006bf185196ae5457d224fca6555068bfb64a228e8b44b4221e85d2f1137224992f41a78301ff768527e953e50424a45d21e8877a16a915629f45aba1aca08769c561260d4d58bcf36af98c26e6b81365b91720f3155c1f9383d8c7ab295aabe6c9799f625d29da42bd4002fbd337ceaf991573678f6384da18541c4e3a3c9472620c19129bb41e2b5e71884b98a8ac319a0ec2be11948f4c02b0b824a2347e00945ec89f45f431dfb3605d8228ce69136867570bd0149f4fe2b53f19d1458e7d6f9688af7c5ec3021fbe61cb331a5a44c9f5e9a4a1192d5eeddf98f47e1c8379bf000ae6886cc3eb5442fa8586652550876a4ef31dd941eac082e9fb1ff26db706159ec7be0a5051b408fc955c55335db1f46c6e87113aabb03960f2f4fc986e4e583021b6b69e7c68b0d1093429630cf7f4a7e895ad45a41363c53d0ddce0804a8858092bc7a069852fff02773e0abd6c7cc7d3d34c0bebfb34e1c5e95ddb184c4a0fc77ed09fca96dc472ba0391214c489890ea410085d4c6ecc69f3facf0b1587372752a421641597563fedc9fc64452d7a7a0db9560ec1c8564a3180a5e623b65e1dd494f967556bde56f9bf58cf5e07a050258e4a90cc2700831a4113391877c65ebaadea8710e23ca9f7afb8c5bd90edf38211b32874b65bef5455159c1a1d17ac3da17819f8adcb254a62d7f4c362cc470f75fddaf6f6624f3a56b0187c4e21295534a832a0f2720c411d0685751fff095078d18bc854856f7e1abe14eb76e9e45fd1eca282804784d9c27fd15ab00bb6dbfd864c401f759d0f2da8ad8640b81066038b72c6cf26605f8388cdd67aeca21dda79d6dd01bc3d3ff5ee29f5e016ea681ba581940d0130685d42e9635c6ca27e1eb9fbf08f44879f4f479eefbde7b65476fbe379f771b0f116a2e6e5f65416b72f2c49452c40b2f108ab86ac7dbb8e252ed32946e5be280b512734c96db9511b21eeec3d0caeac51f8ca315ad7dd62efd1113e03932856d5eef73035a0ae24fef22a2c8aa1db28bc87702c34b2d2d722ddf0ad9d2eb4a6c16b85a7e22c49d115c676afeef8f66ec95ec805e4c1423df5dc2eeece55f107005e15b3ded7c5ff7b7d6fd8530049ede7b776bee594fc18e29a4ae177419bfcb2a0185f51ab35c389baf9b37742192e1c36c2ba7d5c46f683dfa8ca8103a824033c68d844d0ed55e560b96b8421162f57e2daaef5853bfe476903fcb42196e78ae2afe3d5da230de9628e2d268dfc50290b2a8ddae0ade58e2919e0f9be4b538220b885e20b28741cce7c6072aa27cf076197d8e046072045f4a4e20058e7ff428419ce7629b7f76dc0568d99586fc4095107b02228d4f2e9978abe68ef302747a67805d0158b88e94bdb73f1c6e1ca8918b311b0a5d101130e142d2085778d1546532c842f2673cb7a3774eb6d23fa901883aaa682aa7ed0195c3f899f6b485e09715a3791ac9102c177b37465344523fef3e9479ce39d358db0106b5b4cfe26415c5ebc2d00ab36af1171ca33f2c95ff16b63f91e10de4405aec7d6368afc20643616bf30505507f7af84a6f70a36c9f6644bf22bb141f88f15c01f99fbb0344b6db03c042cb8e80e6e38717c3e749cae3782b3d3529cbcfd68a04ade376a7f334ea471d21ebbc62035a57cced3f74a1612edb492a13aacc93d51348249451616f01bbd0e89f46fa53acc5490c7164ef8b2ac0a236b9da37f696db7d2e2dd51243966d2deeae418f2edc6f38fabe746ec5bc832db5af6856266c140e78ce15c2699778910c8e002290b52a68bd14c3be294154c7f448be0160d4adc856b307aa4d3a3ec82af0d25d951d25dc2027ed7861c9ef7d0228179a9fd38ba45f8721d6ce3dd42dce8f58f917d3555c04151cae3e0fc761291c632c0f13e618958cda614f7650efd18e1ffed06bc171530b5bde901becb2b021db47e541679bbb55f7337e1d205e1031eb2ef9a332e84bcbd9b5e27682159c86d3031ab01d741fedf1b05a4e1bc82da108855c8d833abb99821c8be81df68818e2aa094a3cde6f3d5e1bd8b2e86daba12aa2b572ffc81c65c3e498432edb00f1fe6fb04ec92a96b2c206a36a5623f8710c06cc20fdc661230f8b441ce4ebfa45a2890a4a43f2dbd498a9ef9d1f4e748ec81bca42a27aa8acd72406ca303050a32aa644f60e1c58a036ca2b0f0ca69092f6d08a40ee97ff700931b87039bdff71043a75b1578b33d98b391ae0dccbd46f5428cc80016412cbce9532b70454ab801c77072249412a2d49e8f608ab7480b1a9416714c825bd07a96641b865daeda71a5bf6b9e28bbf4a9042e79a6d6dd0c1d99dd3cabf4d580b6bc22999acbfb6f25a33e622104c13aa173e2191eb70dd0db82ae47d1ddbbdb24d3d4403bcaa64bcde88c86ce5bd535694b24f117e729abb3582e2166658f969a206b44aa37837c6efd7f094443c65f43b95826aa97ab1d3dc9aa3e56b31b1d2fe5e2eb44d2b5ccb1039118ac3917148fe4dffdc81daac74007ce71dd5e779be416de62b271bc4379e0a24c6c42e8e8c0213dee8588752d54b12b4da7be2e7a75c6b3e8ad9a92a4768d0611bc91ddd4aebda0cfb84226280ce6f621b5a83016b51bc9de2fe0413ce43ff967cf3680c9e1c359316ab207d93382df330b6a1fff25f01506952465fbdc1d36aeb0124b593b29619b712867c63c7e872d65d18b8834c505ff23688bda7b9d4e4969d6b69aae0b5ff14a152191c5f94a061aa3a7db71bc4ad9ce217a931a92b35e6faec8e00800f96b0efe6d42d1edf25573f12da245539f8fba9ab270cf738d585a144d9098e5d529f3e8662903de413ca9174b9bc6da0a70be23cce8b1dd7b0a1db109605f20d3ba5d72d4361e63cdbfe58ba1e19c0cbb0ab65090c8dc30265ac76a191707804756107d14e1ec41b6b8765286f99960fb601394bb1db089bd5ea19f3b98666e003affb6e9477a42f1c836efdb2355ce392849a777a6c4ac9e1eeb7bad57faa0f25eec2adf2d2d3b20e5ad1ce82ffdd9264e90a37a269f24841742479ccfadb38664c503191da287b16ca59d06b0ebb09e658cc0090665d8ca90b917c4a089b9474b9d76ba7d9deeb96f9a82ed10365d756da05f23bc9f81222b09e5d4b490977052f0bdd3afacd28b2f6730b3a6784eff8653fc8dcca5d17f7c34b05a30cf939ca10c52b714a5ab51b77b523917963a9f3374c7004b81588e06103615fb793dc267e8e3677d8fed75cb371"),
							WithdrawalCredentials: ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
							Amount:                1,
							Signature:             ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
						},
					},
				},
				VoluntaryExits: []*zond.SignedVoluntaryExit{
					{
						Exit: &zond.VoluntaryExit{
							Epoch:          1,
							ValidatorIndex: 1,
						},
						Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
					},
				},
				SyncAggregate: &zond.SyncAggregate{
					SyncCommitteeSignature: make([]byte, 48),
					SyncCommitteeBits:      bitfield.Bitvector512{0x01},
				},
				ExecutionPayloadHeader: &v1.ExecutionPayloadHeaderCapella{
					ParentHash:       ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					FeeRecipient:     ezDecode(t, "0xabcf8e0d4e9587369b2301d0790347320302cc09"),
					StateRoot:        ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					ReceiptsRoot:     ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					LogsBloom:        ezDecode(t, "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
					PrevRandao:       ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					BlockNumber:      1,
					GasLimit:         1,
					GasUsed:          1,
					Timestamp:        1,
					ExtraData:        ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					BaseFeePerGas:    []byte(strconv.FormatUint(1, 10)),
					BlockHash:        ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					TransactionsRoot: ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
					WithdrawalsRoot:  ezDecode(t, "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"),
				},
			},
		},
		Signature: ezDecode(t, "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"),
	}
}

func TestRequestLogger(t *testing.T) {
	wo := WithObserver(&requestLogger{})
	c, err := NewClient("localhost:3500", wo)
	require.NoError(t, err)

	ctx := context.Background()
	hc := &http.Client{
		Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, getStatus, r.URL.Path)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(testExampleExecutionPayload)),
				Request:    r.Clone(ctx),
			}, nil
		}),
	}
	c.hc = hc
	err = c.Status(ctx)
	require.NoError(t, err)
}
