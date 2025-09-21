package stakingdeposit

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	walletmldsa87 "github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/cmd/staking-deposit-cli/config"
	"github.com/theQRL/qrysm/cmd/staking-deposit-cli/misc"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	qrlpb "github.com/theQRL/qrysm/proto/qrl/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

func GenerateMLDSA87ToExecutionChange(mlDSA87ExecutionChangesFolder string,
	chain,
	seed string,
	validatorStartIndex uint64,
	validatorIndices []uint64,
	mlDSA87WithdrawalCredentialsList []string,
	executionAddress string,
	devnetChainSetting string) {
	mlDSA87ExecutionChangesFolder = filepath.Join(mlDSA87ExecutionChangesFolder, defaultMLDSA87ToExecutionChangesFolderName)
	if _, err := os.Stat(mlDSA87ExecutionChangesFolder); os.IsNotExist(err) {
		err := os.MkdirAll(mlDSA87ExecutionChangesFolder, 0775)
		if err != nil {
			panic(fmt.Errorf("cannot create folder. reason: %v", err))
		}
	}
	chainSettings, ok := config.GetConfig().ChainSettings[chain]
	if !ok {
		panic(fmt.Errorf("cannot find chain settings for %s", chain))
	}
	if len(devnetChainSetting) != 0 {
		devnetChainSettingMap := make(map[string]string)
		err := json.Unmarshal([]byte(devnetChainSetting), &devnetChainSettingMap)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal devnetChainSetting %s | reason %v", devnetChainSetting, err))
		}
		networkName, ok := devnetChainSettingMap["network_name"]
		if !ok {
			panic("network_name not found in devnetChainSetting passed as argument")
		}
		genesisForkVersion, ok := devnetChainSettingMap["genesis_fork_version"]
		if !ok {
			panic("genesis_fork_version not found in devnetChainSetting passed as argument")
		}
		genesisValidatorRoot, ok := devnetChainSettingMap["genesis_validator_root"]
		if !ok {
			panic("genesis_validator_root not found in devnetChainSetting passed as argument")
		}
		chainSettings = &config.ChainSetting{
			Name:                  networkName,
			GenesisForkVersion:    config.ToHex(genesisForkVersion),
			GenesisValidatorsRoot: config.ToHex(genesisValidatorRoot),
		}
	}

	numValidators := uint64(len(validatorIndices))
	if numValidators != uint64(len(mlDSA87WithdrawalCredentialsList)) {
		panic(fmt.Errorf("length of validatorIndices %d should be same as mlDSA87WithdrawalCredentialsList %d",
			numValidators, len(mlDSA87WithdrawalCredentialsList)))
	}

	amounts := make([]uint64, numValidators)
	for i := uint64(0); i < numValidators; i++ {
		amounts[i] = params.BeaconConfig().MaxEffectiveBalance
	}

	credentials, err := NewCredentialsFromSeed(seed, numValidators, amounts, chainSettings, validatorStartIndex, executionAddress)
	if err != nil {
		panic(fmt.Errorf("new credentials from mnemonic failed. reason: %v", err))
	}

	for i, credential := range credentials.credentials {
		if !ValidateMLDSA87WithdrawalCredentialsMatching(mlDSA87WithdrawalCredentialsList[i], credential) {
			panic("ml-dsa-87 withdrawal credential not matching")
		}
	}

	dtecFile, err := credentials.ExportMLDSA87ToExecutionChangeJSON(mlDSA87ExecutionChangesFolder, validatorIndices)
	if err != nil {
		panic(fmt.Errorf("error in ExportMLDSA87ToExecutionChangeJSON %v", err))
	}
	if !VerifyMLDSA87ToExecutionChangeJSON(dtecFile, credentials, validatorIndices, executionAddress, chainSettings) {
		panic("failed to verify the ml-dsa-87 to execution change json file")
	}
}

func VerifyMLDSA87ToExecutionChangeJSON(fileFolder string,
	credentials *Credentials,
	inputValidatorIndices []uint64,
	inputExecutionAddress string,
	chainSetting *config.ChainSetting) bool {
	data, err := os.ReadFile(fileFolder)
	if err != nil {
		panic(fmt.Errorf("failed to read file %s | reason %v", fileFolder, err))
	}
	var mlDSA87ToExecutionChangeDataList []*MLDSA87ToExecutionChangeData
	err = json.Unmarshal(data, &mlDSA87ToExecutionChangeDataList)
	if err != nil {
		panic(fmt.Errorf("failed to unmarshal file %s | reason %v", fileFolder, err))
	}

	for i, mlDSA87ToExecutionChange := range mlDSA87ToExecutionChangeDataList {
		if !ValidateMLDSA87ToExecutionChange(mlDSA87ToExecutionChange,
			credentials.credentials[i], inputValidatorIndices[i], inputExecutionAddress, chainSetting) {
			return false
		}
	}

	return true
}

func ValidateMLDSA87ToExecutionChange(mlDSA87ToExecutionChange *MLDSA87ToExecutionChangeData,
	credential *Credential, inputValidatorIndex uint64, inputExecutionAddress string, chainSetting *config.ChainSetting) bool {
	validatorIndex := mlDSA87ToExecutionChange.Message.ValidatorIndex
	fromMLDSA87Pubkey, err := ml_dsa_87.PublicKeyFromBytes(misc.DecodeHex(mlDSA87ToExecutionChange.Message.FromMLDSA87Pubkey))
	if err != nil {
		panic(fmt.Errorf("failed to convert %s to ml-dsa-87 public key | reason %v",
			mlDSA87ToExecutionChange.Message.FromMLDSA87Pubkey, err))
	}
	toExecutionAddress := misc.DecodeHex(mlDSA87ToExecutionChange.Message.ToExecutionAddress)
	signature, err := ml_dsa_87.SignatureFromBytes(misc.DecodeHex(mlDSA87ToExecutionChange.Signature))
	if err != nil {
		panic(fmt.Errorf("failed to convert %s to ml-dsa-87 signature | reason %v",
			mlDSA87ToExecutionChange.Signature, err))
	}
	genesisValidatorsRoot := misc.DecodeHex(mlDSA87ToExecutionChange.MetaData.GenesisValidatorsRoot)

	uintValidatorIndex, err := strconv.ParseUint(validatorIndex, 10, 64)
	if err != nil {
		panic(fmt.Errorf("failed to parse validatorIndex %s | reason %v", validatorIndex, err))
	}
	if uintValidatorIndex != inputValidatorIndex {
		return false
	}
	if !bytes.Equal(fromMLDSA87Pubkey.Marshal(), credential.WithdrawalPK()) {
		return false
	}
	execAddr, err := credential.QRLWithdrawalAddress()
	if err != nil {
		panic(fmt.Errorf("failed to read withdrawal address | reason %v", err))
	}
	if !bytes.Equal(toExecutionAddress, execAddr.Bytes()) ||
		!bytes.Equal(toExecutionAddress, misc.DecodeHex(inputExecutionAddress)) {
		return false
	}
	if !bytes.Equal(genesisValidatorsRoot, chainSetting.GenesisValidatorsRoot) {
		return false
	}

	message := &qrlpb.MLDSA87ToExecutionChange{
		ValidatorIndex:     primitives.ValidatorIndex(uintValidatorIndex),
		FromMldsa87Pubkey:  fromMLDSA87Pubkey.Marshal(),
		ToExecutionAddress: toExecutionAddress}
	root, err := message.HashTreeRoot()
	if err != nil {
		panic(fmt.Errorf("failed to generate hash tree root for message %v", err))
	}

	domain, err := signing.ComputeDomain(
		params.BeaconConfig().DomainMLDSA87ToExecutionChange,
		chainSetting.GenesisForkVersion,    /*forkVersion*/
		chainSetting.GenesisValidatorsRoot, /*genesisValidatorsRoot*/
	)
	if err != nil {
		panic(fmt.Errorf("failed to compute domain %v", err))
	}

	signingData := &qrysmpb.SigningData{
		ObjectRoot: root[:],
		Domain:     domain,
	}

	signingRoot, err := signingData.HashTreeRoot()
	if err != nil {
		panic(fmt.Errorf("failed to generate hash tree root for signingData %v", err))
	}
	sizedPK := walletmldsa87.PK(credential.WithdrawalPK())
	return walletmldsa87.Verify(signingRoot[:], signature.Marshal(), &sizedPK, walletmldsa87.NewMLDSA87Descriptor())
}

func ValidateMLDSA87WithdrawalCredentialsMatching(mlDSA87WithdrawalCredential string, credential *Credential) bool {
	binMLDSA87WithdrawalCredential := misc.DecodeHex(mlDSA87WithdrawalCredential)
	sha256Hash := sha256.Sum256(credential.WithdrawalPK())
	return bytes.Equal(binMLDSA87WithdrawalCredential[1:], sha256Hash[1:])
}
