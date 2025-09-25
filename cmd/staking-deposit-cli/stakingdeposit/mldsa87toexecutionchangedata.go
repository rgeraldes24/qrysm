package stakingdeposit

import (
	"fmt"
	"strconv"

	"github.com/theQRL/qrysm/cmd/staking-deposit-cli/config"
	qrlpb "github.com/theQRL/qrysm/proto/qrl/v1"
)

type MLDSA87ToExecutionChangeMessage struct {
	ValidatorIndex     string `json:"validator_index"`
	FromMLDSA87Pubkey  string `json:"from_ml_dsa_87_pubkey"`
	ToExecutionAddress string `json:"to_execution_address"`
}

type MLDSA87ToExecutionChangeMetaData struct {
	NetworkName           string
	GenesisValidatorsRoot string
	DepositCLIVersion     string
}

type MLDSA87ToExecutionChangeData struct {
	Message   *MLDSA87ToExecutionChangeMessage  `json:"message"`
	Signature string                            `json:"signature"`
	MetaData  *MLDSA87ToExecutionChangeMetaData `json:"metadata"`
}

func NewMLDSA87ToExecutionChangeData(
	signedMLDSA87ToExecutionChange *qrlpb.SignedMLDSA87ToExecutionChange,
	chainSetting *config.ChainSetting) *MLDSA87ToExecutionChangeData {
	return &MLDSA87ToExecutionChangeData{
		Message: &MLDSA87ToExecutionChangeMessage{
			ValidatorIndex:     strconv.FormatUint(uint64(signedMLDSA87ToExecutionChange.Message.ValidatorIndex), 10),
			FromMLDSA87Pubkey:  fmt.Sprintf("0x%x", signedMLDSA87ToExecutionChange.Message.FromMldsa87Pubkey),
			ToExecutionAddress: fmt.Sprintf("Q%x", signedMLDSA87ToExecutionChange.Message.ToExecutionAddress),
		},
		Signature: fmt.Sprintf("0x%x", signedMLDSA87ToExecutionChange.Signature),
		MetaData: &MLDSA87ToExecutionChangeMetaData{
			NetworkName:           chainSetting.Name,
			GenesisValidatorsRoot: fmt.Sprintf("0x%x", chainSetting.GenesisValidatorsRoot),
			DepositCLIVersion:     "", // TODO (cyyber): Assign cli version
		},
	}
}
