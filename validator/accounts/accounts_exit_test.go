package accounts

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/qrysm/beacon-chain/rpc/apimiddleware"
	"github.com/theQRL/qrysm/build/bazel"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	"github.com/theQRL/qrysm/io/file"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	validatormock "github.com/theQRL/qrysm/testing/validator-mock"
	"github.com/theQRL/qrysm/validator/keymanager"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TODO(now.youtrack.cloud/issue/TQ-1): remove test below when ready
/*
func TestDisplayExitInfo(t *testing.T) {
	logHook := test.NewGlobal()
	key := []byte("0x123456")
	displayExitInfo([][]byte{key}, []string{string(key)})
	assert.LogsContain(t, logHook, "https://beaconcha.in/validator/3078313233343536")
}
*/

func TestDisplayExitInfo(t *testing.T) {
	logHook := test.NewGlobal()
	key := []byte("0x123456")
	displayExitInfo([][]byte{key}, []string{string(key)})
	assert.LogsContain(t, logHook, "0x123456")
}

func TestDisplayExitInfo_NoKeys(t *testing.T) {
	logHook := test.NewGlobal()
	displayExitInfo([][]byte{}, []string{})
	assert.LogsContain(t, logHook, "No successful voluntary exits")
}

func TestPrepareAllKeys(t *testing.T) {
	key1 := bytesutil.ToBytes2592([]byte("key1"))
	key2 := bytesutil.ToBytes2592([]byte("key2"))
	raw, formatted := prepareAllKeys([][field_params.MLDSA87PubkeyLength]byte{key1, key2})
	require.Equal(t, 2, len(raw))
	require.Equal(t, 2, len(formatted))
	assert.DeepEqual(t, bytesutil.ToBytes2592([]byte{107, 101, 121, 49}), bytesutil.ToBytes2592(raw[0]))
	assert.DeepEqual(t, bytesutil.ToBytes2592([]byte{107, 101, 121, 50}), bytesutil.ToBytes2592(raw[1]))
	assert.Equal(t, "0x6b6579310000", formatted[0])
	assert.Equal(t, "0x6b6579320000", formatted[1])
}

func TestWriteSignedVoluntaryExitJSON(t *testing.T) {
	sve := &qrysmpb.SignedVoluntaryExit{
		Exit: &qrysmpb.VoluntaryExit{
			Epoch:          5,
			ValidatorIndex: 300,
		},
		Signature: []byte{0x01, 0x02},
	}

	output := path.Join(bazel.TestTmpDir(), "TestWriteSignedVoluntaryExitJSON")
	require.NoError(t, writeSignedVoluntaryExitJSON(context.Background(), sve, output))

	b, err := file.ReadFileAsBytes(path.Join(output, "validator-exit-300.json"))
	require.NoError(t, err)

	svej := &apimiddleware.SignedVoluntaryExitJson{}
	require.NoError(t, json.Unmarshal(b, svej))

	require.Equal(t, fmt.Sprintf("%d", sve.Exit.Epoch), svej.Exit.Epoch)
	require.Equal(t, fmt.Sprintf("%d", sve.Exit.ValidatorIndex), svej.Exit.ValidatorIndex)
	require.Equal(t, "0x0102", svej.Signature)
}

// exitRefusingKeymanager is a keymanager whose signer refuses every voluntary
// exit with err, as a remote signer without exit signing, or on another
// network, does.
type exitRefusingKeymanager struct {
	keymanager.IKeymanager
	err       error
	signCalls int
}

func (km *exitRefusingKeymanager) Sign(context.Context, *validatorpb.SignRequest) (ml_dsa_87.Signature, error) {
	km.signCalls++
	return nil, errors.Wrap(km.err, "refused")
}

func TestPerformVoluntaryExit_StopsWhenRemoteSignerRefusesEveryExit(t *testing.T) {
	refusals := map[string]error{
		"exit signing disabled": keymanager.ErrVoluntaryExitSigningDisabled,
		"another network":       keymanager.ErrVoluntaryExitNetworkMismatch,
	}
	for name, refusal := range refusals {
		for _, outputDirectory := range []string{"", t.TempDir()} {
			t.Run(fmt.Sprintf("%s, writing json %v", name, outputDirectory != ""), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				validatorClient := validatormock.NewMockValidatorClient(ctrl)
				nodeClient := validatormock.NewMockNodeClient(ctrl)
				nodeClient.EXPECT().GetGenesis(gomock.Any(), gomock.Any()).Return(&qrysmpb.Genesis{
					GenesisTime: timestamppb.New(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
				}, nil)
				// Only the first key is tried, and no exit is proposed.
				validatorClient.EXPECT().ValidatorIndex(gomock.Any(), gomock.Any()).
					Return(&qrysmpb.ValidatorIndexResponse{Index: 1}, nil)
				validatorClient.EXPECT().DomainData(gomock.Any(), gomock.Any()).
					Return(&qrysmpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil)
				km := &exitRefusingKeymanager{err: refusal}

				raw, formatted, err := PerformVoluntaryExit(context.Background(), PerformExitCfg{
					ValidatorClient:  validatorClient,
					NodeClient:       nodeClient,
					Keymanager:       km,
					RawPubKeys:       [][]byte{{1}, {2}},
					FormattedPubKeys: []string{"0x01", "0x02"},
					OutputDirectory:  outputDirectory,
				})
				require.Equal(t, true, errors.Is(err, refusal), "got %v", err)
				assert.Equal(t, 0, len(raw))
				assert.Equal(t, 0, len(formatted))
				assert.Equal(t, 1, km.signCalls)
			})
		}
	}
}
