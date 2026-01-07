package attestation_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87/common"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/proto/qrysm/v1alpha1/attestation"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

func TestConvertToIndexed(t *testing.T) {
	type args struct {
		attestation *qrysmpb.Attestation
		committee   []primitives.ValidatorIndex
	}
	tests := []struct {
		name string
		args args
		want *qrysmpb.IndexedAttestation
		err  string
	}{
		{
			name: "missing signatures",
			args: args{
				attestation: &qrysmpb.Attestation{
					AggregationBits: bitfield.Bitlist{0b1111},
					Signatures:      [][]byte{[]byte("sig0"), []byte("sig1")},
				},
				committee: []primitives.ValidatorIndex{25, 30, 17},
			},
			err: "signatures length 2 is not equal to the attesting participants indices length 3",
		},
		{
			name: "Invalid bit length",
			args: args{
				attestation: &qrysmpb.Attestation{
					AggregationBits: bitfield.Bitlist{0b11111},
					Signatures:      [][]byte{[]byte("sig0"), []byte("sig1"), []byte("sig2")},
				},
				committee: []primitives.ValidatorIndex{0, 1, 2},
			},
			err: "bitfield length 4 is not equal to committee length 3",
		},
		{
			name: "Full committee attested",
			args: args{
				attestation: &qrysmpb.Attestation{
					AggregationBits: bitfield.Bitlist{0b1111},
					Signatures:      [][]byte{[]byte("sig0"), []byte("sig1"), []byte("sig2")},
				},
				committee: []primitives.ValidatorIndex{25, 30, 17},
			},
			want: &qrysmpb.IndexedAttestation{
				AttestingIndices: []uint64{17, 25, 30},
				Signatures:       [][]byte{[]byte("sig2"), []byte("sig0"), []byte("sig1")},
			},
		},
		{
			name: "Partial committee attested",
			args: args{
				attestation: &qrysmpb.Attestation{
					AggregationBits: bitfield.Bitlist{0b1101},
					Signatures:      [][]byte{[]byte("sig0"), []byte("sig2")},
				},
				committee: []primitives.ValidatorIndex{40, 50, 60},
			},
			want: &qrysmpb.IndexedAttestation{
				AttestingIndices: []uint64{40, 60},
				Signatures:       [][]byte{[]byte("sig0"), []byte("sig2")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := attestation.ConvertToIndexed(context.Background(), tt.args.attestation, tt.args.committee)
			if tt.err == "" {
				require.NoError(t, err)
				assert.DeepEqual(t, tt.want, got)
			} else {
				require.ErrorContains(t, tt.err, err)
			}
		})
	}

}

func TestAttestingIndices(t *testing.T) {
	type args struct {
		bf        bitfield.Bitfield
		committee []primitives.ValidatorIndex
	}
	tests := []struct {
		name string
		args args
		want []uint64
		err  string
	}{
		{
			name: "Full committee attested",
			args: args{
				bf:        bitfield.Bitlist{0b1111},
				committee: []primitives.ValidatorIndex{0, 1, 2},
			},
			want: []uint64{0, 1, 2},
		},
		{
			name: "Partial committee attested",
			args: args{
				bf:        bitfield.Bitlist{0b1101},
				committee: []primitives.ValidatorIndex{0, 1, 2},
			},
			want: []uint64{0, 2},
		},
		{
			name: "Partial committee attested - validator index order",
			args: args{
				bf:        bitfield.Bitlist{0b1101},
				committee: []primitives.ValidatorIndex{0, 2, 1},
			},
			want: []uint64{0, 1},
		},
		{
			name: "Invalid bit length",
			args: args{
				bf:        bitfield.Bitlist{0b11111},
				committee: []primitives.ValidatorIndex{0, 1, 2},
			},
			err: "bitfield length 4 is not equal to committee length 3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := attestation.AttestingIndices(tt.args.bf, tt.args.committee)
			if tt.err == "" {
				require.NoError(t, err)
				assert.DeepEqual(t, tt.want, got)
			} else {
				require.ErrorContains(t, tt.err, err)
			}
		})
	}
}

func TestIsValidAttestationIndices(t *testing.T) {
	tests := []struct {
		name      string
		att       *qrysmpb.IndexedAttestation
		wantedErr string
	}{
		{
			name: "Indices should not be nil",
			att: &qrysmpb.IndexedAttestation{
				Data: &qrysmpb.AttestationData{
					Target: &qrysmpb.Checkpoint{},
				},
				Signatures: [][]byte{},
			},
			wantedErr: "nil or missing indexed attestation data",
		},
		{
			name: "Indices should be non-empty",
			att: &qrysmpb.IndexedAttestation{
				AttestingIndices: []uint64{},
				Data: &qrysmpb.AttestationData{
					Target: &qrysmpb.Checkpoint{},
				},
				Signatures: [][]byte{},
			},
			wantedErr: "expected non-empty",
		},
		{
			name: "Greater than max validators per committee",
			att: &qrysmpb.IndexedAttestation{
				AttestingIndices: make([]uint64, params.BeaconConfig().MaxValidatorsPerCommittee+1),
				Data: &qrysmpb.AttestationData{
					Target: &qrysmpb.Checkpoint{},
				},
				Signatures: [][]byte{},
			},
			wantedErr: "indices count exceeds",
		},
		{
			name: "Needs to be sorted",
			att: &qrysmpb.IndexedAttestation{
				AttestingIndices: []uint64{3, 2, 1},
				Data: &qrysmpb.AttestationData{
					Target: &qrysmpb.Checkpoint{},
				},
				Signatures: [][]byte{},
			},
			wantedErr: "not uniquely sorted",
		},
		{
			name: "unique indices",
			att: &qrysmpb.IndexedAttestation{
				AttestingIndices: []uint64{1, 2, 3, 3},
				Data: &qrysmpb.AttestationData{
					Target: &qrysmpb.Checkpoint{},
				},
				Signatures: [][]byte{},
			},
			wantedErr: "not uniquely sorted",
		},
		{
			name: "Valid indices",
			att: &qrysmpb.IndexedAttestation{
				AttestingIndices: []uint64{1, 2, 3},
				Data: &qrysmpb.AttestationData{
					Target: &qrysmpb.Checkpoint{},
				},
				Signatures: [][]byte{},
			},
		},
		{
			name: "Valid indices with length of 2",
			att: &qrysmpb.IndexedAttestation{
				AttestingIndices: []uint64{1, 2},
				Data: &qrysmpb.AttestationData{
					Target: &qrysmpb.Checkpoint{},
				},
				Signatures: [][]byte{},
			},
		},
		{
			name: "Valid indices with length of 1",
			att: &qrysmpb.IndexedAttestation{
				AttestingIndices: []uint64{1},
				Data: &qrysmpb.AttestationData{
					Target: &qrysmpb.Checkpoint{},
				},
				Signatures: [][]byte{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := attestation.IsValidAttestationIndices(context.Background(), tt.att)
			if tt.wantedErr != "" {
				assert.ErrorContains(t, tt.wantedErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func BenchmarkAttestingIndices_PartialCommittee(b *testing.B) {
	bf := bitfield.Bitlist{0b11111111, 0b11111111, 0b10000111, 0b11111111, 0b100}
	committee := []primitives.ValidatorIndex{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34}

	for b.Loop() {
		_, err := attestation.AttestingIndices(bf, committee)
		require.NoError(b, err)
	}
}

func BenchmarkIsValidAttestationIndices(b *testing.B) {
	indices := make([]uint64, params.BeaconConfig().MaxValidatorsPerCommittee)
	for i := 0; i < len(indices); i++ {
		indices[i] = uint64(i)
	}
	att := &qrysmpb.IndexedAttestation{
		AttestingIndices: indices,
		Data: &qrysmpb.AttestationData{
			Target: &qrysmpb.Checkpoint{},
		},
		Signatures: [][]byte{},
	}

	for b.Loop() {
		if err := attestation.IsValidAttestationIndices(context.Background(), att); err != nil {
			require.NoError(b, err)
		}
	}
}

func TestAttDataIsEqual(t *testing.T) {
	type test struct {
		name     string
		attData1 *qrysmpb.AttestationData
		attData2 *qrysmpb.AttestationData
		equal    bool
	}
	tests := []test{
		{
			name: "same",
			attData1: &qrysmpb.AttestationData{
				Slot:            5,
				CommitteeIndex:  2,
				BeaconBlockRoot: []byte("great block"),
				Source: &qrysmpb.Checkpoint{
					Epoch: 4,
					Root:  []byte("good source"),
				},
				Target: &qrysmpb.Checkpoint{
					Epoch: 10,
					Root:  []byte("good target"),
				},
			},
			attData2: &qrysmpb.AttestationData{
				Slot:            5,
				CommitteeIndex:  2,
				BeaconBlockRoot: []byte("great block"),
				Source: &qrysmpb.Checkpoint{
					Epoch: 4,
					Root:  []byte("good source"),
				},
				Target: &qrysmpb.Checkpoint{
					Epoch: 10,
					Root:  []byte("good target"),
				},
			},
			equal: true,
		},
		{
			name: "diff slot",
			attData1: &qrysmpb.AttestationData{
				Slot:            5,
				CommitteeIndex:  2,
				BeaconBlockRoot: []byte("great block"),
				Source: &qrysmpb.Checkpoint{
					Epoch: 4,
					Root:  []byte("good source"),
				},
				Target: &qrysmpb.Checkpoint{
					Epoch: 10,
					Root:  []byte("good target"),
				},
			},
			attData2: &qrysmpb.AttestationData{
				Slot:            4,
				CommitteeIndex:  2,
				BeaconBlockRoot: []byte("great block"),
				Source: &qrysmpb.Checkpoint{
					Epoch: 4,
					Root:  []byte("good source"),
				},
				Target: &qrysmpb.Checkpoint{
					Epoch: 10,
					Root:  []byte("good target"),
				},
			},
		},
		{
			name: "diff block",
			attData1: &qrysmpb.AttestationData{
				Slot:            5,
				CommitteeIndex:  2,
				BeaconBlockRoot: []byte("good block"),
				Source: &qrysmpb.Checkpoint{
					Epoch: 4,
					Root:  []byte("good source"),
				},
				Target: &qrysmpb.Checkpoint{
					Epoch: 10,
					Root:  []byte("good target"),
				},
			},
			attData2: &qrysmpb.AttestationData{
				Slot:            5,
				CommitteeIndex:  2,
				BeaconBlockRoot: []byte("great block"),
				Source: &qrysmpb.Checkpoint{
					Epoch: 4,
					Root:  []byte("good source"),
				},
				Target: &qrysmpb.Checkpoint{
					Epoch: 10,
					Root:  []byte("good target"),
				},
			},
		},
		{
			name: "diff source root",
			attData1: &qrysmpb.AttestationData{
				Slot:            5,
				CommitteeIndex:  2,
				BeaconBlockRoot: []byte("great block"),
				Source: &qrysmpb.Checkpoint{
					Epoch: 4,
					Root:  []byte("good source"),
				},
				Target: &qrysmpb.Checkpoint{
					Epoch: 10,
					Root:  []byte("good target"),
				},
			},
			attData2: &qrysmpb.AttestationData{
				Slot:            5,
				CommitteeIndex:  2,
				BeaconBlockRoot: []byte("great block"),
				Source: &qrysmpb.Checkpoint{
					Epoch: 4,
					Root:  []byte("bad source"),
				},
				Target: &qrysmpb.Checkpoint{
					Epoch: 10,
					Root:  []byte("good target"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.equal, attestation.AttDataIsEqual(tt.attData1, tt.attData2))
		})
	}
}

func TestCheckPointIsEqual(t *testing.T) {
	type test struct {
		name     string
		checkPt1 *qrysmpb.Checkpoint
		checkPt2 *qrysmpb.Checkpoint
		equal    bool
	}
	tests := []test{
		{
			name: "same",
			checkPt1: &qrysmpb.Checkpoint{
				Epoch: 4,
				Root:  []byte("good source"),
			},
			checkPt2: &qrysmpb.Checkpoint{
				Epoch: 4,
				Root:  []byte("good source"),
			},
			equal: true,
		},
		{
			name: "diff epoch",
			checkPt1: &qrysmpb.Checkpoint{
				Epoch: 4,
				Root:  []byte("good source"),
			},
			checkPt2: &qrysmpb.Checkpoint{
				Epoch: 5,
				Root:  []byte("good source"),
			},
			equal: false,
		},
		{
			name: "diff root",
			checkPt1: &qrysmpb.Checkpoint{
				Epoch: 4,
				Root:  []byte("good source"),
			},
			checkPt2: &qrysmpb.Checkpoint{
				Epoch: 4,
				Root:  []byte("bad source"),
			},
			equal: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.equal, attestation.CheckPointIsEqual(tt.checkPt1, tt.checkPt2))
		})
	}
}

func BenchmarkAttDataIsEqual(b *testing.B) {
	attData1 := &qrysmpb.AttestationData{
		Slot:            5,
		CommitteeIndex:  2,
		BeaconBlockRoot: []byte("great block"),
		Source: &qrysmpb.Checkpoint{
			Epoch: 4,
			Root:  []byte("good source"),
		},
		Target: &qrysmpb.Checkpoint{
			Epoch: 10,
			Root:  []byte("good target"),
		},
	}
	attData2 := &qrysmpb.AttestationData{
		Slot:            5,
		CommitteeIndex:  2,
		BeaconBlockRoot: []byte("great block"),
		Source: &qrysmpb.Checkpoint{
			Epoch: 4,
			Root:  []byte("good source"),
		},
		Target: &qrysmpb.Checkpoint{
			Epoch: 10,
			Root:  []byte("good target"),
		},
	}

	b.Run("fast", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			assert.Equal(b, true, attestation.AttDataIsEqual(attData1, attData2))
		}
	})

	b.Run("proto.Equal", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			assert.Equal(b, true, attestation.AttDataIsEqual(attData1, attData2))
		}
	})
}

func TestVerifyIndexedAttestationSigs(t *testing.T) {
	rawSig0, _ := hex.DecodeString("c5cc7d99c51c724dbbaf549ad4e30ff352e8b45aa848160f7f66858962bfe2c55ca54585ebe57d1ab07c035538a1c1003885fe87bc3f154bf3e8a83541aa4a32bfba3f66a729c1db091e1e6134f613feae2ac35bd9ae432fc860efcc63ac59ed6d99c0dd9cec89ee167db8c68d4c03de7c05ab54131df3b8c272e79d978ea93dd5ec62ce098e1135466ad596b2dbdff578b31d8929d6ffac8d0cff681ceb710c4890a59b3bcb5e35691223d0232d81ee412094501764d4a0b07566d3770761ecf48d0d714ffce2976928da8db5fad7d3dd29fd65e7cb164b7fb677d1696856715d611917c99ffd69c8fe26a8744e513b63c22b3c607b7040f48454b4c1ddda6c9da66f6aa9167b0482955a4caeee37b84a0dd7eb6225228c3a902adb0739991b46235e235ebb777fec55b79d20b852f8f336ba7efb4c75d62ec64d10bde4c2eb0c7e2d3de78b83294f0dccde18da623cdaeaa0d1410f8b24a1bd8031f89cd673c47eb8e00b73aa422b8ced8a08486d4fdbf31a17e942bd9aa291beb3999fce5e68f75be5f26d1ac69be89beb4eaa3a82eec8f6621ea272ca2f6405dafe2d9aaa1d985db5ff36820f55517505d07430d5e7d56521fb107269aa86f410b7e29d4d7fa1d80976012ff8bfd4f1c423051f85e9a8103ccc2e23e7f1d319edd2f9cffd8607890c49b3cb41e066a23e7d8559ae1da0e3a2cf489787a5c88862028a3b32dd2b5c7e8e1a7b866a35a176855932f68633efde8e47bcfc87f955b3f73e0f9ead17a5b37dcfa3ec21191eb2de0795b01f8b3593cd828512c9be4f783e1af02cfea87f6cdbe8c70390262aa7ec116aa0e08b4e62b57e577d688b40827c0e53c5a942eee69b157ab11e48068f3c5d166638bef21f06f3dc6c65752a5d7b74f9895542d978021e8ba5936f58aaabe948d42cd2a8e7c902bddf7695a510613f5832394e183875431483914ebb40a0dc7af6077670abacc97d53eaafddaa3b5b1b99be4428c5ef9ec2de74b1ad0478cd193526e3ffd8fb8c4df2bb34746c3df41a6a3b722b43c9c9a93f00c5931326824b9368f6d5b82e99df636144dc67855abf283be1e5bd290f017d98cbb21d0d10882fc7ce1db564a5f2ad29a291492ce8a7e0686e34840da20f9cdce076767ab14437fd711cb7924de6f45b9f900413fb8195ebcdf2b1039d938d19e882d52fe103fdec8e6add3f6fadc2055eca7d7698894ea9aee45577ab58248591e681ba38598884f18c53427d983b3b70e6c3e5bca40fac97a1f8419e9f7472971bf950dac03491ff6f3229d9e34c354fecc5182e8cc9fb249cf001d65c2d26462a90fad3d0aef17b7465531b2c8bab3283b8aca264c166e53d30a4b236963a8a2b178e21d1dde58f3d84a71b966d23e5741185312463dac6d6ae36a546dea083d6380d6a32638b6de74543677a9f7bc39e064ed4cef68a64ac71724bb15c8e6d1629199f167898c5d37f296c5566fda092db374d84c5d9b7258db31e82e1f9969c8d1266a50a6ce6cd6e73f9c774d6172c81599df163b94776cc9b3342dc63bbca7893ef21e90aeaf9489f12116013f562ae728f7f599f2cea951a9bc0143f8349d6a3ca13faa580709d8f8f09bccd462ccf3a438b79edc0754e657d533879696514c84b108affcf22cb5f46f605de945bd646953cabaefb6248105940355c0af629592c00e30c818f99b965fb301b9a5dbcf8f2a2bfba2e2a53fdbaabc2ec49b76dc8b1356ca27e7e2e1ca860a02e9e6a18fd358a86aa810a314cea126c9fe9d1a2b23d7a50b662c799c4c4046115914730bb494276bd7d526f71ccda36801eb11871ee239b8750d9c0320818bb045ba07c7ce5268a8721c893d98b053a9a6c98b29cd33c774636ab96feb020914265342d6407c2945c45364e836f57db7d2ea3fb5a71609f599886eae45dd16b1a7fb93b83aa20657716fab4285d2b6653a165869337080e3502851166656357cc121e2af93db1269dfad51a8a82ad6084e7fa07189bae7947d0bb33be97bc90ed73094e33267fce4fc6635b5e54901e0364c35cef2162c9fbe4c2d33115fc77936129f94ee76f3482be79bf41c8c5e97d783033b01f57b54be5ed4feb58d5f9066a7f58392c7812b9602d493a6a26a8ea1cb511fe99c6f9bfa6d88e43f0ff9008856e45be0fd8e0f7acd9eef447f6b7399cff2d99b26f671b25daee12d9d934efcd3e018de34b9c281bd62a3721f960a5ebe11b4ea86f768ba55b3ed3b42144b9f0f3e17a9f471a676e6eb91faccc2b3711f3ae9b7aba090a831147a2eb66f9230ff4bfb2fcd557c0fc16bc538d0348bcefae816248122ad47bb20300a39ead08d715ccacbbc2983a0ca9eab2c9c633768affb1dcbf68208c1af8e65702930b7d9841da1f2e2e932c7f2ff36366b17227185a28766bd3e64792f9f8d16e4b022113e80574666a951d601d4186853503f73952d7e3740f17c5e67dd91f6cc8238e554828432cafcea758efd9ea0234e29e9f25deacfab2894458f565ced9fcba21cf6b20084abb0db362c0ccec342af2cb0233ef15b9a4e9b251921f71c917b1c458dca9bacc276a96691af5a01beff4d59a69896dc9a675ea8e4a7d28ebd751a30e5c19079173882043562f6d6c18d47f0a3218bcefbaf769ac84f36c1743c88d55d2436cca043918d1c9eaa6616954d36fa4b3b7faa45513280bda0cc2004214b718c3bd28db5d36fdedd8ffaa1696f628f1adf62936046db8d12bd03ec93d590d73e49b1e54d22009156b6238cee5075fc3ea201bac93b10e0728965d4c5641f994ea9d034826535517742d408864a6581ac51d0eae2b624fcc3c1849edcceb157dc03c5b9b15097abb0ad0534e104bf83c81f0e226b376242cd06deb7841276389476648c12af7a3177504586f356300cc46acd6592d6ece229879c57d75ffeef91049717d15427ea64eae5c39b78e6051221271e422d11cd485d8380d58284958b55be767622859b60da77cfcbc1dcee15073a41178516de0f247bcbb7ed815040ca870070e6ab5baa88f15f948a47ed21b1ad17a537fa96c0c9e21b62b0ef8900237fb0dc7356fc60c66a31f142c0154b7d38bbb35f3429e142ec5a656d57c51faad70cb3d8d7c6b6f07fe52a23ac54f4ec426eafd2d9b1d986cbb124d9dd2208b4eb74de4f88155dbcabf8f98f9ad00a28cd40f930a3504a8324e875770d557c3ae6cb1a4639987c75ac745735866058c5083bfc2e7d1d62f8724cc889e49735b0869d15db37d5f7f035b46433076ebfe7de244d502096b0129624a9f6a8d62a014733ff715e60b6460f92a698abd2c7c7e39c29deb7f10e55d80f44da6d25464cb0a5d9ca9fe1116272a06525a90d604f47482a6b62da4f4c38df53c00e4a353161933fbdc7177b4943d5d7aa93d54638665cdfdd9947a5257b84e6a39cb22f7f2ea1962ab2aaa5c196441b3e573a09fcc6f3e41e1b32121985aa3e748bf1fc700c0174aed50b23a129374ca8ba854df9e8f05774f44f7083f82cd153bfef27cc59f4a6f6bbf4f9b1d0e0369d4816111c226387ff9aca7eba7853e25047989732c2a14a5af45cda843a27f0f7cca4df35528300978e515da1c310240bbcf3f04359867978b9944d6553050db59baf5c766c561f91a4a8243fe9d931381c0aba86ff0fc5c4c072421e9710822f3faad337faf7aad4100bcbbabeb8ba895827d8b0a500a2fc0fab8fb9390504a1ac811d03bb0af70b1930bf24c0297ed4ee16196df8b1c7983c24c09f516d6a83799877762003b8e9c78aa736a6b1e02623e8edd6f91951f9869f322fad73672a766fcf118432b3db8fc98018f4fd8c7a2f4ceecd20f98181e12d7dd7b8028a476e7f827ac2bc134272dc7415fb233723049de0ef45774725e376d4a949f220bcb1fb771c4b7fe806ee159d8faf665ca3f4252181432c6e33f10cbc29df0ebd2be0f643cb6b960d041120b54f598c5ed69b7d126be297896dba88e90135b61af96773db28947b278d3e0794d215ebe4f3b3a84a7444537478773ad6adeedcb7450d8a67409f74d92f44185920ad7ebb48c63755dd6dd35d94bbb47deb27a3337940c7a0586ddf7cc04798103e81943ffac5c5018ea62c81edee673813aa3805ab82c109413ed83cf6caf31bcab63ff5a754c9a09391f09863a28aa1e38b14504ddba8ef783de04c7fd7653e6c8101461d1442b33b758439b4f909cc619c8718337c00e8ed24ec1b6554cb8b0963e6eaa2031cefc58343a3f06274d3cae4ef9e2c299315f2331b6765fc819d4b12d05bb992d132f9920bf516fde0c0b78fbf7738a235f854546091c927199d798af84a805f8ba022ea34b9a05dac11cd99f12b1d76bb522798c6eeae9262f3cc7a72db9dc801591e6ab217175a2af1fa56a38aa4606c523627a0bdc32c2c0b9e35f002e8053617491e844f8c9f8931c61b18b389691962a5865ccab534266f52e54c20fac8da5a1914c0c86bac84dcf3f351f53b0ae773f86c61262e2886a7e36180e918588e92b60422494adc2f969f6f2163767f4eb45c7dfb4063ab5ac04cdca66eb4a6f6332bdf660070e2d9109c89350ce2335848eb4ab79c8f0c92dc4d99019de184e02235caff50a959bd4c5689a811df9743fadce9ac919d01d74c630ceea78bd7d1f90483d8a329ef2d502426751e99b312b36eaba061b775432ac0319b603775ace95fc8d21b19b14d05d67a2cb923f499119040b319ae1b901535290ad9bf591b34084007d6ee2cef2057ac8bb297a480c119d644ab34963c402e1fac4c4accf1f3a1cc1c905e5f9712130d599339c48e75706498bb266f54a5736c5a0d8906abd11b3011bda7d49ac6ea2d4b00b9eb9e1eee808c93f860d8de1f0f97ed9452db480ebdf9a47fb80fe764a6ae70665429d2365fff91539e6bee27b7622d1862dd95ba3283d5ed7cc985ef44ebbd6a5392f8cb7e5898564dcaa44bd4aae4a404e97f3a24c5fb31a8913dd0ff40c7f90cbe2a5d04b5b36be647f9c2cb1c02ff678547404f7aff29d9380027470bd8581c93182f73fdae66a63a40f477db0ce435d24c5fabbe89817ad8a5569450524097d117869cc1d43ea1656ea1b4ddcfe7755fa095bea64df50be09addc73458cf9c7d8c5d782b119f3422e8cead7b4e471553925e2d346136ee0e4faa64a3e250eebfc2780c35237018d1a6c8803537d3dc87ebed68fd20b566339db0bef4481fd557ca243d15ee8569a6453a38af555ad4c7ac46893c0882f642b2a0a7ddae8f249f4d4c789852750eb4602a1e1fc2d536e1e993a8e5b78f0116e672ef49fce4247a7a4aa65f99531796efd2e1c25db4a1cc72a3a45140de75017b776541a19504e3b37e34b2970b1c2552d94756dfedf7c47a5c0138c99b8732067ad7b203a2e2e2ab388ab66b7c4f557d61a3ee6b51f16359f994f8b8d06fc002b2b02bbda9240d467d90c241e67bd55f058533197f48a89b3fc93b57c2691ed191966498c7f6ebbd40f39d08f25436762499661ca16db59e8b4ea66ab4a6cbb76939622e69eb3a44aa31cb3f58e0006139083b66d368017a65dc417e713e7b63791f29877cc99e1d34b42647aa5b9d73007bb7ad59f400aed80939ed2c6b42146bf563b2f1e5cb57ac5bcb9d59f71e1d92f4dec8f6706541a65f6c8b33cf222e2949861ff0ee8fc4cb72219001cf6fc0c48f41ac68ee35bef50414f28719fcd9c40ecb40d5572bcdf29e7304d22cadbc054a5ef03fa5e2e6f7ba5935984a383dbac8f794fa3109e62c48e683513db2531564fc48364cc85255e44bb99c53892198c9d0201e5c2964030671ef33953fc8d3ed87b97c5b8d39b2223b32cf3e4f105c601a53510c7147953af91dbc98d2faa94e32df9acf584d44f45fbcac620a9a6525949cfb0aeefebf7359056608bd1a963524d6769b569a5bd5a249985eda868984a9b6696052674fa9299ae615164ab378201e0661253250f86d293927fa030fa017fc295249f3146b3c3020316c82cf36cdbb439a1d80435e9648769e067f94ea2fb6083ffa1248345eb2e44845a6eb0a8dbfabf89a33f368d3789d942a841730a607d7e48c91f3f9a23108693ce7701fd9fb5df0e6ea8a318f7a63b569064401ac6d4f7d4ae6c29cebe6cad491a518ce82980f12a6bde07656c0b589707369fa838268c99161790a7baf0bbb20b98c38017a93404fb7f8eb594a3d1221878bd4e02c38da0b0908fa96430e869c0c5be9e08612a6e9f5343ee9ace30e01e07c01c46042b06b9ddf9923d925ba4cdd155caf0db63dd7f42891b2605ced344da642be2abebb8ae526f29cbc1d409e2f0d7339ded080f4374e31274c1f208e0c7d1fa10de23631a35af874d030f26c0fc9806cbd72f50df8da90a0da3a55f9b2d58e3f8e5c763d03474d988b1fde89eea0cbc212ba21cc90a94b51ead093b656775797b9398a1a31c246686a5dfe6275ab727686fcde7223c44506c76b8c7f0f81427333f69808fa6d40f233e61f12f38697f9cc5cdd9e4ea0000000000000000000000000000000b12151a242d323c")
	rawPk0, _ := hex.DecodeString("d2d7a04f06b477907a62a7c5ba0b8d5bc0755354d7dcfed5c4268f324aad7afa3763f3538a0bb6fb4e03106bcf9282b8f06a14d10d2fb325b2dc4aaa729bb9f96b6f93fd26390bcd4fae8ab22f76c9e7f8d2eedd5f4d07061cfde53ee02a9125e0f49336357ee96c0b1494befd49e4771977cdce1a22b445213d8a0af9c6c69c110f6e37ed4589387fbf693f7ccabe46512c2fd5845399790c545da71cc36fbf37f84f810e76d9f6d2e545ecbaea2e182616ec6394229b3c5d2b476f4c31c45c19e76bfce9b101bc9360e400920b75280915e2d2fd3ecabb458c66bd414efb3cbaa82ec03e9f4bfd723d16da088dd0bcd6b62e1cead240eb630b594d1793be1a0dce8e6832e3bdf7767a94f32a096a245a46e8580d137ecc4c12dbd2f3ed7b0fed2ffd9d28e391b959a869401d30e84250ac4d1ca476488430a5a44770fa043c9118701553e8c84bf2c70c0b9ca3e7a4836a4881e754b2a59fc0a35af7f8bd6b8d4abb4d5582e8ec8be9f7ae3a11e32718d07dfc2b518e1075cf36087c6167dc344f91e240f8c02335c8df200c2bad5010f50a1501a32786035c2877ddfd4f74f01fe93d07ed2c4405ac198ca50c2fb2f61e0552a48aa4c177d2ee76b02629764146d98b58af4751d140fc40ca94240f85b43e7bb45a1d9f9728398437c56b9023c2d248011dc6ecb8b9201496212593204722ed2d8054e9f1de6e13c9509b4984a928b003088430001abd0257cde9352fd80cbad728b8645ca7e3d722d5a2ba211a5bebf7adba676a06d4edc7284e522b19412d31a1de6c2e09e0147759cb3554a68bec189783a9ff6fff194a5325db68e1d4903415d4ffb19f18ddfbfa51568b77d55461b4443ea10202d7bc4121dd391915c09654110239b73a4b209f31d86fe50fad3602b8fe9cc6e6fbdb441ce43a6d56ccc11e39e1353c987e5af5f34b0439d3146fca9c8067e4b6ba53abb23ae634f25ded6509fb76293a5fdb1d32671cb87df5981ff2970f816e2011eaaa6ec810a9bcc670d959b62d79df9ae3e8643237d41600df6a3be8c0c8e3ef95e1ce0cdacf0fb2e24e44e3b279ac7e97c433143714c269923acc644634da6abfef9f3a19bbce029f5e49a63e645fbcf08771829e6996faa4f551142a3a2b6bd647b498105b537a9e829351824b873f0acc568e26e554c987ed21ee302e00a3b6177d1779101b0f2945782cc95df387d14425de4180b401d9fe3516697162b119f0d181c03d1449199ec40f03545ce67997715a749085bac6fffa005ba6187130e831af6e41cf2ff2be59cbdf0b3076e6aaa41eb8da4a08920f68b024f41816e35e7e492ecc0b964fbc11a6761e463da7a5943447b44ea8d1d491cd2fa1ab5d0d9f2a3f3719ed3781a961a85dfc94a4539926af967003cc1ae18b4401e448a4da38f91adec2aca6abf445921008d7bdf3e0da818afc64f13078852eeecf8f291abfd5a992d2a82284dc36ca9f3201de67087ddead3f239fb2c0afa240fc451db64d159c0dee372a5f4f2374e22edce0137521400e4831f8164d90991f3eacd106154f24061c6c1a757ff47aeb21421a93626680c61d808c359c14e4c4be92b0fe0a24590c5a406e9e2b6730c458510129df62aa281339b1d117fe38d62f530042b16eda0a3389ea8bb1d617bf0e3e75d03d2cced188184ad89ffb044a91e7bf18b9fc95a48cdb2c9863224df03a9cdd269c2bfae5f312de03f0bde5d6f47c270a679a49af372d96ec84999195ec584cb8a19793f764f314f117d470091b7ff1ac8bc0a1d53b575e6bee6cf76b8eb56a9ac73b8f925657986418a21db2cdb0f5f3b604884ac53ed278f49976859ebf3ab51b4a7d1084a145757f0853579b7bd9b4c0233308edae2848f6c3b3fa8268fcf8e4649c587a89fbc492f2ab9fe2aa8ba62db6e1ad64315d9d974169926f7a45bdb518c4f58f24b4c6b9a45bf78725a1ff218648300383320f53af620c548b91b3f05ee705b3892bd7788c28151483fb20d2bc042f4691a9476524ead501114931b561792f34003be31d75a246ebd1b32214c9b307cc5c01b20f4f53a8b3c1d41b5c62b0754e7efc11e4f0f0d602c09ed00b4aa4c2f5bd07b9adfbd67748f6a1c144f088513b7f963d8be5c9ce5faa0a8bbbdd6406885119d2606de058af4bb721f6d3894487effd64b2b1eaf7edb37572a5b0715e5e1ba0967c1b591d5647a9ce581748ae1c7e86ecb9787918d232a37b4d6f03bd4462d96b839c171581f1c3e7ad689775290bae38e3f18c1c260fb61b23790af68fcebcc94dcdd3191521b1c87b3b7f2f503d8024b9bb8498a470b1dc9ca56eb1708803a408b6d850a3090eee75aba9b407caeb63250ab6e796778b04466f055fa1272df81e21dfff99a780b8737aada488de893fd6f5943ddd5481f6538ca8c1d7853384919cd719d67dd68adbbe5b539c111e7efe95e01cc83fef2a1458d81aeb0c3cd5e4fa3338f10ebe34dded123f612e99cbe4bd2798702c11e65a5e5d615c3e6b6ffec22ae3b8594ee1aa674d44db73a035e7a2214e71e0173f4381794e00fdc14550300ce1fb1fd0a2af6fd57e593d03bfeadf63c4fff0b3f14e9af9d716d4ccb012d248686881f324df220a671957945ba5d8e4411513fb98913608b86a3d17e39125d9fc6e4004d03a03157ca6e4ab885d39edfceba4cd0be32cd94f7a45903d1291ebcabd20a6bd9291228e8b9ca473db08ffe50cd8066323767239d1c780756f12e68d3db383fd9e02407b9c4cfd3d15c1809eab0d3f67cb00d4e994634164c6c0a26337ae6e39c2067b48662d3b35c49e359c0b4bdeba2d167d4e649634e2d2e6413bd3fdf68abba0cae31abaea0596b664d573497ef038843deed7580aff064b900a67646a0eb090dba30ff748445780e88188679e4f1249d8a56f795e31b7ad08cebc9277492a5d774c596ede41dfe67dd7c4a56c1f3651927259d830a4a46aa9b6accbcb58713466f0f14e28867f97df163ea8420a55077b521e9953c2f52049d21530a54b70b992b4b07003829f92ea1a0a91ac3cc186309ba3011aba87272eeebd4e578992a57a9644240752556067fdfb95ead63e29faa2d581851938c76b38fb19f7337e642bedec496eb2fbda65891f9181986bc8317f5f6e2829839fe05c2b44c675a94ccd3296628bc4bb728dd788b6f939da7af1a5e161650931a8e16bbf5035f0769f018c4075d1f107512d9bca93f98c716b93080cc61194dbb9e245132c711fef64442b6d75c3329a9c46b249b1dcecfc8e93bdfa7ce439f66afadbd5fc7b54c86e4783d2c6c397a6b3c482ccffceaffe22ed6c47c27d972a4529cf58c5b18ba50aaee96be9f5c0412ee9bd7e0a5bc0848a519762591cbaa1aaeeaaa06e07424bc7d21f8a12046a7d13cb08ca106984c456a82d9c2d91e188e30d8b6cf62546ff6e46eaed60d1b4af9ae45780524f5bc74e44708f2b995169559153d0d8127841923240543682fe8274b95519615ce375e51bf06dff7f3a6343b6f40c69d6f2b989c8848419c4cff4aa09734c7070ce50d3fc9248208cacb0be1325c226fbb906a32d742eb81be7e92da9b9ac9fe572ea4713524899d70c3ef4d60fa87aa9783e58b043a35e96e436b547a55e91699a21cd76e1afe459bf2c9ac60bf58dd25c11c3")
	rawSig1, _ := hex.DecodeString("d28ece6c42f69aeea812217aa016a6e3cf10b1b4e1ff5c7119008a239f41f27bfee37571568d46e59a82e3133df9979077056ea58bc3a9d96f55533b6da4efd18a67eb4d9b247e0ccb97c7d0c003a873a00e97c44443dc8624163f9fcd380bd30909ca448ccc3b6a6aad1b1adc474a676563ac338da49c615602b0b97f64c6b2b1d1223192dae83490d213dc84fd04a5f3c9c905569f0c214a66f90101aa0fa55f38f1b16d90863fefeb9112dfaca68e73658be59e14901f7d0973f2b3199fb8dba41332455a8310165ecb2f510f0ae4bfa5d9fb8f2cdcdf587dfa4d6d0beee6a5cb62f937a4a2ce8562e12cae9161d454de2cce5998c6ea84f9a33bcd32c3b6d157d87b81e8f673d89dd4322a0b2c700e0c4513efd1dfe1ea94390a2f9a378c085d424b0fdda85dc2415bfab276e3a881968272d2b41b9bef2316cca1a1838876bbe5cece9b2b39eee60507f9b0aa1793a6fd6a743da50cf85e4b704177cb97fc62c357a9ace1c1d3b54391e94bb3c248c9c1889078f5196d053be58749bbea91d7ea3b62973dbddb7dc6bc61fd8b840ad953f22d299e4a9e505b9dbf581a3172abadb95894ee4abd7e07e1f7cf7e11fa837a324adf0704b09ae08d253b040d31df12a77c494f6e064fb73128b3d42da5190c0733dc701f6b4dedc8500aa1a12c85619bb899955666ede2952943d52dfa917f0db3401e8d600add584d08b3c96eba8201496fb2aae611ecc3939f75ffe666e8abc6345e67c1059c7eba1259c355101dae56e4c8cb53ef3835d27365e989bca67699982f2468fab0fb725ceea70924cb81b2e3a0b170eba59dc523215df902004fbfce54a71cc0521c9412bd0513c3f33867453f4103fb204ab823468e4dcc38adc045b17977b8266dd4d7540b0a236e5b3a95f6004e3b9a4bf662269d8e6160b1d69f1364651583894abc0735a1c8d2a11867898ca869053b89919098cf088ae47329d68f48fe3f9dac30b843128ca232d5068abd220c308bfcdfe135c26508361c512dd5c8d4468ebeb512102a8eac0580ce47a61066417a1e86bfbfa53da5a1b5c9d81147f7afcd7fe135c71d462c4541d09a21d23668c99ebdf86bbfc5e913a9d9c7ee61b43d47f57804bed87c1b21930a4a55677cad909d7625dd3085be96d07774cd6da07b29575515f923769909faa2337f1cd5d969a64396be72e44adf15ee52ae9e5a253e494d7ae1598cf004adc94b3d2af2a40d308525be4bc68f0d3fd3ca98485e96593c111375a3f71fa926e31fb25f33bc3de8f714f1b87aa5f72585047696ef11ae07e10c990bc064c1e66e61f180c03881af7d8b3e4ed636ca4535da81ca03bfd4cd551ab279745dd074ac6fb65e8834f94d7c04dd529dc8c4f52a76a1641ffa5cc7b74d0f19c31543753ab50bbdce4319015e06f3ad0bf91e97221cd6e6b0a85806fb6aad9abaff16477ae1da827b1a6b18b448f0a8f3c674d519797ac5446833b1a6d7c2c665c3391d59db258c5a90c1b63a8627eaec9ceb5dedd1b32df9b84938ec19a170303766b851431989f4a78884156b5c20cb509befaa77e289348ad9137af29d51f876ca69580350ef006d5f96f17a1cec07284ea09ca78fbe8aa091336178129e65f1735b4ee7b282275149c2ac79e3da165fff6fcc113f580cb2334ce43a5a5ddb59f791e8e7787bd343628ef7329c8f539086a904658f30752ffdfb0ea74b3c4bbf4c49c7a659ba8e0dc62e543fc185013135d53826b0e126d63e5787bde0b40260c5d5e3fd8361ce00db2cd60c8228af5bca7d9914ecda2ac6ad6894d799348a162d2c43beb02b3f8e46e2995106cda0780bdf95fdbdaff955934d18d0f21e5bc936076c077ff85fa6609f6db153a2f995113a9e8bba407b50f834c3bdcec68395443f3828a73795b65c7d1cef54dc1bc20bb4735f89ee7a35dc679406e14d06d9f090bbfde24853000a3525cffaaac9ac9782bfc5cf2fcdf93887c80a8f8749d2743ad41c6a1b11d480fe77c3e47d5dd2901d4a029f69d5de97cfa54f061b2d5d1d41e068fad192d77c8696f876e97c40b109da7866d832d3442b76fe382fe74d18568d8b314784df60a520296aa0d95c60aba0d83e399436e0f27d4552e900250f6afdcecc705a38fe3f4cebc35ab8cc563efb1d62d6fb557d6b7afacf6bb43c634f57cc1dd5498164799ab583226330b0d17288f631cbc74a392b9dfe7da71b5b875dc6548c9dfa03521103599d093a05a41681e4c18a14f110ef7f2823a176cf49d2627626a95f5c3ebf725044428555b5773d2f4d09af592326791467d3fa7fa43ca4f1b52676a99f7eebfb79eb1e6e7f1f3cee241577ac7750eddaa806ca91b5613370f838f8753d32f2f016a5f5cb55013439ebaf6a237bc6813a34aa8102b6a13ef5b967a1279f2ddfbba74a1983c40b342192c36019e9b43aa345c33d000ef5b8919ee8468f33aff553a3c0123f7dc8594133125d731dc20b99e003232e0900ba03883d5bbf86741c3c18cea6d62ecbbbbd2efd5b501128bc24409c6013b0577f512a1c21a7fcbcc954768a9d520c1e406c6e326aa6ba4963ff1f0c9e4cb54c8004085f24d704fd9ab1e6864dde20c41a03ff70f98d35d91957a28fb685593ffb1ca4a837a752d54cd43ffdaf24207ea40652dede5306148ea78972fa728d562d68b242e03b86c9bb9cbbcb441f4b749641b6fa81adc4a1ca3ac8581720af2d3b9b6ea87161d5072d1f2e3a3fef500bb79b504a2999b3b6b67db1cbf77356644db7dc931dcefc53ac5feeeebdb59f70108618f271ba277791a980ab2a32b30f9554cb975de8d449c84d4598ca6673b966a0fe37ae218dca52d406446d78927907d36cd1212df183f4a7cb4edfdfcd3614c744d1b32e47d088369a9e26f4da4ad700be66593a4ad32b4810c4b748ca66a1d1f559e6bc6a4009742ba9ae21b8f5ce8324c17120fd53ac79caf9dea2b023fabf3b3cc52c323a3351922226edd9ca0a70173f934c76f108d8a202de488ebf36e8965cc53e1243789f3ef644e20c1b5cce2f3f2627caf51f369eabc106506f0cc0a0938a79a2b9ad11e55d83b2f5fa66f590c65a3f0fc14bfdd2b79afe2974ea45aa47ebbca84f89524691a52b2fecf734a6e0fe4750d2ea836ce6b322cb1e3f8c335b4d34a47e076af91b5b0114a2eb3783ade54addde653d9fb3fa9f149d78f66f31fbf12516409f9867cc43258d410474a0c77dcb457e8870b7debb72846d001cc6b9dd376fc0bd6d7fd9e9beb58ccc8eb7b9f54302d18d72db91fabe308e39b585f6fd0cb4bdc70b4c70b3cab1a7f9458e007a130b8fd0267b9e4fcf2b37c70cc1e50d8cb21eea948555fd8b0dec7f98722d86edf9de37a8d006700e5b767dc4c0c5016e3cd7f9c3336c75a124107d58c3bfb3dbdf0e1a93df829bbb8315c57b739217cd4af7a0bb65eca651b8a86f1fa857104de56eb6dc65c9ce725b65a5a004073bfbb60632a22685f2e97fd2069d12b9b2afd7db63ea33b06fc5b4a137a202ce34368f927f93a4bd3d2d124d330cbf6210abc5e41349522f85f1a77c7b3c2d9ff8dc1eca9d39d886c71cf41fc968d153de041019c808ad49560d76043e3581ee6a8ed9d9906c440a3af86a1a05018c9ffd1a691b237733a2ac742af26cc25c8262565d4f60a07ca4cc189c0ccdafb66ec485c13c4ec338fafeb64d1a59171d3c4f5fd1e061ee5bb3b8be1df7497b22a7341f6df5b2f2d2b3c76a41d06347c1222a72ac1eea6e41701e533e0eb99e60e4e38275e4204753b2cda3f00744247185afea8f236d21cb71e6d1a8f88e1596d663dff8062edaf2ea51587ee507b013d19ee080771c08502b58aebd9e113d262b2abf65bee6e80626b3f0e4fecebfcbc229c7a950324bbb77c678e12c68ac88db9d11ab99c677ace5275732f772d8a20c3b8d145bfc990fe3fe54ff9a7b2bf7ae26a7375577fbc0ffadb51a47c48bc66becfafb85ba477f5600a0f0214f0a7a1333c7cc2eca67b95a6f4fab9dd9c5dd916e131418bb3ae8ac2c0713f81ab25e2b9960293d9aa29cd108699abfabb19a5375aec6e928c07f180abeea379753960065e8ef1d8adf14f44aaa918b0221613b3058aeb4e58a6403cfad7543a1222d4baf8d360b4b9565b94d1dca5b8fc7ba982d6186628bbaa5e4ec746ab46a5a5b948182f9a1c8ebcf4e58e17b1fe5adb1fcd79f989fc215b3bc4b37ca9e89cfa4fce35f10e413ddca9fe59f109fb0028f12098defebdc63e29e4814d998da68bae5f467cbd6a57f7fe299546eea65a8b7060355574cbd28cb423fd14350e1f5af522d09ca0150a596f987fa05535d041ed6b48af8b2fa70598c6c842e266b4d4f1dbdb412c939abfe9f0add492fc65b3d2def75233e9ab4d258a993f00ac21df494875acae3ad90e94f42f751e3df8efc627cf0a956100895a5e7cd6ba4601c8021959633909de5c5b0c9017a03bd5311a62087659e4026663aa69422383e6b52079575c9331bf9dcfadce0a6e0dc9c16a1a90b823db73edd446ffa9b0b4696f462243fcc83c3529536383d6c86b1edec6b0e2f3bf9e858a1887cdd7b8f2128ac13b336f5fd7972d81d4686a084a868d5a7a570149c104c742681fcee886e7c9d6915a4cf8ea8fbdcb6c418e351267e80ffac4a7fbba0ca04be7b29ddf6cad33db000373765cad80f1d1df8640d984dbd1f8bb6ddeb9089baaee79cdab8ce9e0da088c335d5b6e4ea4d5a428f8ae8894166b0997f803d1576288f6cfd722af96009019e8c4d4695f890fb992a1b54c325b5c404e23eb14423c4578980595e09f6ce20d05c93a46f636cfc14eb1508c1c14d85218b3d97dd590836376a43d4ae53b4463b51f47dd9cacd8fc4376ec175ddbe4eabd8d7bdbb9d6b16b17ab3e11d89b57e96c678e069a876f29bd63bab7b99a1f0c69f75bc247ac68601cabcbbea1171baae735b83f3535ac4db7b4879760aeabcbd9791f5e3ee6775ce61eb7853de72ff900783e354d8ad65b288e9b992d57664bbe5efcd17a70a02daa9bfd7a70bd99a18188fabaacbb3f768e8ad26bb259ed1c670b90039bc5ac51b9d46637a2e25be9b0de24221aabeb22fb3901bf0bd624f921b6b303433e0f53b87d460d73b176f334719a6931654f51b3d6e53a4f3b10f1ba4d742c3f3ee0dd67b6d48b78c61ddf9619bced3145e7c8746343c379984f642f2a809fdfc8fca1f8fb129f96678c8306fc32e3bba74c459e7c4a4d10d477521803eb774fc3af6479a9940c1088b32e7f512d78d2d3c91aba0f725fb86c26193d808d06102266180b71a981b9b2f90218f393ecbf27a15802e5e0774ae72eb70e9016fdb5983e294ed7c18e9b856b7e808a953f36a668746f1a058be973c05fe2fa1fa292cfbfb23fe061b675939f27c658cd51eec7969ae2a62e0dc64a1df573666d8ea2b7bdd350790f34071077aa3d1acfed5f33aea0eb3c6bd13ca866cad57bfaa36bdd184139eff459f3d43eec8003696f4fb7066dfa48dced4e75853521aebe24ce64b845a3e39cb8976e3323dc7f033532ef3ee2ebc38753db5f24b4d9eaf90bf73eabf0257011afcfdc63cd6c16219be9a1d0517c81ab499f3c1ed62d6091fe3e5ba5aec1809d2baec3318427f268f1696703a247e455e70dd01165feefa789e575bf0cd4dfa6b9c49175e81461de545ad11c272c0b4f76fe733f374495239846a352c5d28763a7b979b8ff00f84fe38eedd8ac4dd94f812dccae490f162a52848ecc38d5bf66a9489e0d905949e02690007585a07dda6f8b5b11764a76ae450c1129f2655efceea670471f92a34ee0ebd11d335ecd0ef4eb0f52e409d4c34d8e77c86660e7ad1af00f9186a1c2fb162ffd38213c107e4a72b35686223d10434d5a5bd4cea8045c3d72bd86d5784f1168f05cbb2fd50c7c4a100f832e6bd8d92058799709dbba389a7db7d83f50be80302517639d62ad6f6ecd7e1ab7f37158fbd0a0c0cdf22d35071193d2f48abd05dba793c7dd7e7c579e06278174fa69eb836a70cc786117a5bfbeb322e8805d075d59d08a32854b00fe1e74e7c14a7107677c8b4d9e11ce30758a819052ce5263afe746b6e07e17a98a20c8b7c524fe71c4d51e51d75648df097c10d87f3338f20d886f738820eb5ae17a8c41d8f2a11597847f44a84fbb98753d7c3ac876f47aabf884d97b5efbd907a982838ef50df599527faf268de015881cd109f73dd67747a0103cd04e218c1ac4e5f860a57ef8c818ffb89224bc470b4ec13b7b50f7a91d131d7efac5e7170cf4c3fe990a39cd30e55e1663d967cfbadb36a419caeb90abcf0935e78202f69247916adfa8b9f568bdcdd15b2150518f3dcd2899914505aee71acfe6b87153318ad962b742f147b61331da43788e0c60bc2930dcbed89122f09a4ec6cff125938b924211dea604b848688a7c1e11f6e73838aa4d9f210141c405a76a3f70b3b4b515884868d9bda0735c2181f20223542739da0f3fd0a6162ccf7054e8187d0fe0000000000000000000000000000000000070f1721242f343a")
	rawPk1, _ := hex.DecodeString("6c5c0256b7e67cb7e89b75395082c19cf08a31bdb0cddaa0dbf9099951f1eaba6b78effac950c20e2f10a07e53303e56ca0228685effffd57eb45677935753560cc2301fcf56c73a80684609704b96a50bb25921492b1fd20ee2dc796d4a9f0c6d1b21875c28081e722470236636ab762f8f7b3451f1c5d1865cd6bc75f9e063a93921f4c263413956931d4e4b951bf0c0915f130f32e9334efdb6df5e761a981e0be8708c07d4948643f85862adecad1008ddd0fd166e7933230d8683944959d99f22b61c59542e90caa92c43a1dabda2259029fa798d2aa4226af07ebf1de625603d173833064141bafeb367f2a5df2c177f229dacc119c5a94649a897a01289d4e0d4ace80868153c96fcdb03b4f3c8aa8fdd6b8ab74366322454e400fd2301e30912f6dccaae2647bba0f4b2e893c50f24ac81fb2b9f2fd6ccb8f4e6e1438054dc0284650ba99320654140da53bc57b927bdea089557661056290d826c547483839f8ab77467d959ea0a4bdcbd955b96e27e1d775ce35550f5dfcd8c4347609b75f8fc9746ceecd2da06f36803988a9bf902ccb27c1602333cdb9ff2e4409a7993fc5b54d5d57d1de56564e0885f1d71ee7144df744b0960f753d973d832cc1b7b761282a366abc3a42866b26b8c84518fa05d6a152f017e9bcb466575a470db29f5d7451ff3ae88ff706e9b71e2da68b4eafb82cda58b7ca208685e4c4caeab1ba4a79622e85c08826effe2224f2f16bde95b6a7cccde9b469e9b991e4ea8fc3f1eb54a8fa719af71d321d7b893e3751173f9ef2c48200695f1e29b17597dd5450dd43b3a04042c38f2c602c88ffa4e3e96f1611a43ff6c6734b63f685a9818c9354b900c2d2474afae0728e69e0ba93a9d8c782011fb51e7c6728fb940fef69701a9b32e8527c0a47f8a70f896f5700219215e78cd776ef5079fb6ed9caf20f93c50e20a6e3e7bff8c9b7bbd800e1e59139a3ab3aa5d4018dcbc7095b0d017b77185cc5400693992b1ce633b8d0406b3ba97573108cff5a897b06992bd6385ab5f3537f20d607788d4f0f956103506a0337295915fbe02851b9fcf32e8013a9b433c3250f072571b148436c898e6e78bed671c37fc467491254f68debe627b114c5cffeb1935312fe5abeb4ed22771a35f4aae9b64399908d4be447566e4bddb5ebf2b3837a9b99a7dc59cadba535a77814a43000464cb35a2998688fb2ad2eb66ed367807c9d92e9d776ad2895fdaf12d37cd8c61ff46d027d9adfc0316ec5f3b52f80af2614b18f1d7bdbbda4dc35ba5363d960399042efc1a1fd41f860ed1422ee84e73dc12f1f2eb895d59660c11b1edb72509e5b2212716b055fe52e5a58c3e9b5d22c6d9dc4ca2a1f205d65bc4f9f24f44c72cf42d9b8b91c65858332bb24324b47877aafa523f880cd4724ff990cd2b079105071df7ee90692f43ce23e71a8bc0745984e652b65e6013385e81d44563491f546b0064e9cfb7643da4393cf6b3a7afd4fec232016700c65a8531a88e213a9ab5546255528fc0ac9992c46c1f2ac3192adc32423dbe052dbe33e46aa6d369717c74da5a8eea8e8e281b1f37c8352d2879e4ca9b8e3e0ea3454744ba8bec5434cd3ade1c783085a3dfeaeb90703cca8d0b9fcd9ea063b2ae88ded6b84f9a1498252483a5c7583756631da66bf81ec480478204731baff7f2f2e8cd6ee8a2a76d1dbaed7a8254eb002cc18fd65b57f7982176242e546fda668458378e52eb25ccf149ca097381a878643a34c811d4391474d5d09f5d4317815fc02095cda18372d72ed60d35b70c6554bb91dd8ea4a948ffbeb1bbb130b1109cf4d1702070bb8c9572674fc1df859d9aac67a541af321aa518b1d0de130621141f6be4ffc9226afff782cd2ebb9b1f87162cb32c7df2ee2931a55dff603dda6f81bc1858ab48044c1e6c45c3740c8386d8c357ec0d9dc0ff9ddd7f38ebab0e9f6e6d0f23c944a059109d332e35d2b786fde3df1975b761cef5d52df5f837315c1218d9003ea916f3dd98f926eec8781dd92e726752095e2388cac1a3c8b41da90af4942f8e734105d3b8cba0f310dd38267d2d53e0aa00780efcebbc267ff9bebc980918bdd1b4f42655d0671de9b762ea98ec55fd5d07bb102aeec85903f6653f47ee4951213170bbdcf35a276e0fc4a10df4077b14e9c326890fc502dd5422fc601ad76d45e2ac2f9cdcf1802b4fd2f54af989b0c4087feb032dda3fac5b292af1fbe53013acf162d0a7cfd908d7be37d78904161a04e6f67a63350936ba9d0a94b9400777a3cf2e5c78fc9ea4d0b2de1bd80bed2f0b2201d78a45f37099f912da6698f6bc1eb5a98bf11bd7f3a06fe95a7211a52410913d5ba04d1f80ae24fc499d380980b324ea617def1fd7b90e15df83cd0233f9cbf603aeb41f8f2f3d796596edbca55d385e2620542ecf4fddcee771a69642e7c7bbd2b20acf2774f3e9b0e9adae01ddb5925350b9977ea53071b0db3306dd6dd7c667225d540595013398205381501203bb05bf7c44738936014e1ad5214e2f26059d11f986bdc234f9da8860548e1ede1ca0bf87a010b503f2346610137bc7df2295bb5de4d38c5dc73a7738294232cae9effb82e5a7f012f7b15e223925fb9a1b998545efc28b76a2d9361d545d603cbcc019f70ff073d16c5e2727f123d95eb42339ebac494ce7f28555aea065e6161486fde260a02024f15af6592892934e9fab09025c0ac606e0491e965b58d1dcf60058059e4726728e037a698cf74bf2f2f0f4f9bec51cbc035f32eb3e617cb892af5406e6f3ee74e177a1bb07d1d3b5f80cba842d6d2e7eeb5b2ad7bedb0d9f5e6255033b1bc187064b19b72a2eb926961be1441426200ce27893e30cc217e0a8a9c33fdc22e7c94b49ef094443fca1e528035ec04c83f187686d22862ffcd9edf223d2d1d86da65a597f9f1b784395541bb5d960c1bbc34d0c15404182b3746f875b06bd5f4d41ccb58ed03de6df566cb85595b21ce3bd7062a515bc365c925965e8800263731618f70258a731e41eb45a7d8e19cf18ee8ff0eabb0dd7c89e0f6aacce42dfeee14704d53105f81b627fdb6399c23d330d8ed169e4a76c3d4158016afafdade3b725bfbfcdd8457eef2a8f8566908da236e3a0b35486e0cc979bdcef27329d354624f346a308470d813b28d204b7fab4613243b37ccf92b347390d1da0bf902cd4803a9aa54fd46fc85ff63cae54ce45785aff19cf6b1cf7ada4f3fdaeafc8989e7c9221efc0d5048c6b873bb9951729f92aa097393726a1e904460eb6344a7f4c8d680c0bafdd2f80ac559d82b4970153b87fb7724ac9c92a8dd35c8ebaff1f3d99202d8456d3ed4fa8728f1d4901c883e7cb26dbf804871e4170b6e274c1514d5a64b3c8f4bc70ee41bd0bbc44c1ae016e9bcdf6e71158d37623cf40a64cdf9fb4c230ca8680cd74d49de200acb00ea38117dd8cb246d421b29c355de0cee7b0de003e3c55821a067d8b0b5f8b01a4e5634e48131578a89870e059fb4794d45ba0713179a545a2ffa43d92f178dcf7c6f05c80aca3a6b16419f8d9f6d118caaa5a59535b3211b8ab651de3444ff1205c38df1f7d6b6bf96e04a7d2141a248e0b39619f3da001c437ea752d9c1605d4c11950ffeef401dd3608f368b4e365e9")
	pk0, _ := ml_dsa_87.PublicKeyFromBytes(rawPk0)
	pk1, _ := ml_dsa_87.PublicKeyFromBytes(rawPk1)

	type args struct {
		idxAtt  *qrysmpb.IndexedAttestation
		pubKeys []common.PublicKey
		domain  []byte
	}
	tests := []struct {
		name string
		args args
		err  string
	}{
		{
			name: "nothing to verify",
			args: args{
				idxAtt: &qrysmpb.IndexedAttestation{
					AttestingIndices: []uint64{0, 1},
					Data: &qrysmpb.AttestationData{
						Slot:            5,
						CommitteeIndex:  2,
						BeaconBlockRoot: []byte("11111111111111111111111111111111"), // 32 bytes
						Source: &qrysmpb.Checkpoint{
							Epoch: 4,
							Root:  []byte("11111111111111111111111111111111"), // 32 bytes
						},
						Target: &qrysmpb.Checkpoint{
							Epoch: 10,
							Root:  []byte("11111111111111111111111111111111"), // 32 bytes
						},
					},
					Signatures: [][]byte{},
				},
				pubKeys: []common.PublicKey{},
				domain:  []byte("11111111111111111111111111111111"), // 32 bytes
			},
		},
		{
			name: "missing signature",
			args: args{
				idxAtt: &qrysmpb.IndexedAttestation{
					AttestingIndices: []uint64{0, 1},
					Data: &qrysmpb.AttestationData{
						Slot:            5,
						CommitteeIndex:  2,
						BeaconBlockRoot: []byte("11111111111111111111111111111111"), // 32 bytes
						Source: &qrysmpb.Checkpoint{
							Epoch: 4,
							Root:  []byte("11111111111111111111111111111111"), // 32 bytes
						},
						Target: &qrysmpb.Checkpoint{
							Epoch: 10,
							Root:  []byte("11111111111111111111111111111111"), // 32 bytes
						},
					},
					Signatures: [][]byte{rawSig0},
				},
				pubKeys: []common.PublicKey{pk0, pk1},
				domain:  []byte("11111111111111111111111111111111"), // 32 bytes
			},
			err: "signatures length 1 is not equal to pub keys length 2",
		},
		{
			name: "missing pubkey",
			args: args{
				idxAtt: &qrysmpb.IndexedAttestation{
					AttestingIndices: []uint64{0, 1},
					Data: &qrysmpb.AttestationData{
						Slot:            5,
						CommitteeIndex:  2,
						BeaconBlockRoot: []byte("11111111111111111111111111111111"), // 32 bytes
						Source: &qrysmpb.Checkpoint{
							Epoch: 4,
							Root:  []byte("11111111111111111111111111111111"), // 32 bytes
						},
						Target: &qrysmpb.Checkpoint{
							Epoch: 10,
							Root:  []byte("11111111111111111111111111111111"), // 32 bytes
						},
					},
					Signatures: [][]byte{rawSig0, rawSig1},
				},
				pubKeys: []common.PublicKey{pk0},
				domain:  []byte("11111111111111111111111111111111"), // 32 bytes
			},
			err: "signatures length 2 is not equal to pub keys length 1",
		},
		{
			name: "valid Indexed Attestation",
			args: args{
				idxAtt: &qrysmpb.IndexedAttestation{
					AttestingIndices: []uint64{0, 1},
					Data: &qrysmpb.AttestationData{
						Slot:            5,
						CommitteeIndex:  2,
						BeaconBlockRoot: []byte("11111111111111111111111111111111"), // 32 bytes
						Source: &qrysmpb.Checkpoint{
							Epoch: 4,
							Root:  []byte("11111111111111111111111111111111"), // 32 bytes
						},
						Target: &qrysmpb.Checkpoint{
							Epoch: 10,
							Root:  []byte("11111111111111111111111111111111"), // 32 bytes
						},
					},
					Signatures: [][]byte{rawSig0, rawSig1},
				},
				pubKeys: []common.PublicKey{pk0, pk1},
				domain:  []byte("11111111111111111111111111111111"), // 32 bytes
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := attestation.VerifyIndexedAttestationSigs(context.Background(), tt.args.idxAtt, tt.args.pubKeys, tt.args.domain)
			if tt.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, tt.err, err)
			}
		})
	}
}

func TestSearchInsertIdxWithOffset(t *testing.T) {
	type args struct {
		slc        []int
		initialIdx int
		target     int
	}
	tests := []struct {
		name string
		args args
		err  string
		want int
	}{
		{
			args: args{
				slc:        []int{},
				initialIdx: 0,
			},
			want: 0,
		},
		{
			args: args{
				slc:        []int{0, 10, 12, 26},
				initialIdx: 4,
			},
			err: "invalid initial index 4 for slice length 4",
		},
		{
			args: args{
				slc:        []int{5, 10, 12, 26},
				initialIdx: 0,
				target:     4,
			},
			want: 0,
		},
		{
			args: args{
				slc:        []int{5, 10, 12, 26},
				initialIdx: 2,
				target:     4,
			},
			want: 2,
		},
		{
			args: args{
				slc:        []int{5, 10, 12, 26},
				initialIdx: 0,
				target:     28,
			},
			want: 4,
		},
		{
			args: args{
				slc:        []int{5, 10, 12, 26},
				initialIdx: 0,
				target:     13,
			},
			want: 3,
		},
		{
			args: args{
				slc:        []int{5, 10, 12, 26},
				initialIdx: 2,
				target:     13,
			},
			want: 3,
		},
		{
			args: args{
				slc:        []int{5, 10, 12, 26},
				initialIdx: 3,
				target:     13,
			},
			want: 3,
		},
		{
			args: args{
				slc:        []int{5, 10, 12, 26},
				initialIdx: 2,
				target:     11,
			},
			want: 2,
		},
		{
			args: args{
				slc:        []int{5, 10, 11, 12, 26},
				initialIdx: 3,
				target:     13,
			},
			want: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := attestation.SearchInsertIdxWithOffset(tt.args.slc, tt.args.initialIdx, tt.args.target)
			if tt.err == "" {
				require.NoError(t, err)
				assert.DeepEqual(t, tt.want, got)
			} else {
				require.ErrorContains(t, tt.err, err)
			}
		})
	}
}
