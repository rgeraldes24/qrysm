package zond_test

import (
	"math/rand"
	"reflect"
	"testing"

	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	v1alpha1 "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/assert"
)

func TestCopyZOND1Data(t *testing.T) {
	data := genZond1Data()

	got := v1alpha1.CopyZOND1Data(data)
	if !reflect.DeepEqual(got, data) {
		t.Errorf("CopyZOND1Data() = %v, want %v", got, data)
	}
	assert.NotEmpty(t, got, "Copied zond1data has empty fields")
}

func TestCopyPendingAttestation(t *testing.T) {
	pa := genPendingAttestation()

	got := v1alpha1.CopyPendingAttestation(pa)
	if !reflect.DeepEqual(got, pa) {
		t.Errorf("CopyPendingAttestation() = %v, want %v", got, pa)
	}
	assert.NotEmpty(t, got, "Copied pending attestation has empty fields")
}

func TestCopyAttestation(t *testing.T) {
	att := genAttestation()

	got := v1alpha1.CopyAttestation(att)
	if !reflect.DeepEqual(got, att) {
		t.Errorf("CopyAttestation() = %v, want %v", got, att)
	}
	assert.NotEmpty(t, got, "Copied attestation has empty fields")
}
func TestCopyAttestationData(t *testing.T) {
	att := genAttData()

	got := v1alpha1.CopyAttestationData(att)
	if !reflect.DeepEqual(got, att) {
		t.Errorf("CopyAttestationData() = %v, want %v", got, att)
	}
	assert.NotEmpty(t, got, "Copied attestation data has empty fields")
}

func TestCopyCheckpoint(t *testing.T) {
	cp := genCheckpoint()

	got := v1alpha1.CopyCheckpoint(cp)
	if !reflect.DeepEqual(got, cp) {
		t.Errorf("CopyCheckpoint() = %v, want %v", got, cp)
	}
	assert.NotEmpty(t, got, "Copied checkpoint has empty fields")
}

func TestCopySignedBeaconBlock(t *testing.T) {
	blk := genSignedBeaconBlock()

	got := v1alpha1.CopySignedBeaconBlock(blk)
	if !reflect.DeepEqual(got, blk) {
		t.Errorf("CopySignedBeaconBlock() = %v, want %v", got, blk)
	}
	assert.NotEmpty(t, got, "Copied signed beacon block has empty fields")
}

func TestCopyBeaconBlock(t *testing.T) {
	blk := genBeaconBlock()

	got := v1alpha1.CopyBeaconBlock(blk)
	if !reflect.DeepEqual(got, blk) {
		t.Errorf("CopyBeaconBlock() = %v, want %v", got, blk)
	}
	assert.NotEmpty(t, got, "Copied beacon block has empty fields")
}

func TestCopyBeaconBlockBody(t *testing.T) {
	body := genBeaconBlockBody()

	got := v1alpha1.CopyBeaconBlockBody(body)
	if !reflect.DeepEqual(got, body) {
		t.Errorf("CopyBeaconBlockBody() = %v, want %v", got, body)
	}
	assert.NotEmpty(t, got, "Copied beacon block body has empty fields")
}

func TestCopyProposerSlashings(t *testing.T) {
	ps := genProposerSlashings(10)

	got := v1alpha1.CopyProposerSlashings(ps)
	if !reflect.DeepEqual(got, ps) {
		t.Errorf("CopyProposerSlashings() = %v, want %v", got, ps)
	}
	assert.NotEmpty(t, got, "Copied proposer slashings have empty fields")
}

func TestCopyProposerSlashing(t *testing.T) {
	ps := genProposerSlashing()

	got := v1alpha1.CopyProposerSlashing(ps)
	if !reflect.DeepEqual(got, ps) {
		t.Errorf("CopyProposerSlashing() = %v, want %v", got, ps)
	}
	assert.NotEmpty(t, got, "Copied proposer slashing has empty fields")
}

func TestCopySignedBeaconBlockHeader(t *testing.T) {
	sbh := genSignedBeaconBlockHeader()

	got := v1alpha1.CopySignedBeaconBlockHeader(sbh)
	if !reflect.DeepEqual(got, sbh) {
		t.Errorf("CopySignedBeaconBlockHeader() = %v, want %v", got, sbh)
	}
	assert.NotEmpty(t, got, "Copied signed beacon block header has empty fields")
}

func TestCopyBeaconBlockHeader(t *testing.T) {
	bh := genBeaconBlockHeader()

	got := v1alpha1.CopyBeaconBlockHeader(bh)
	if !reflect.DeepEqual(got, bh) {
		t.Errorf("CopyBeaconBlockHeader() = %v, want %v", got, bh)
	}
	assert.NotEmpty(t, got, "Copied beacon block header has empty fields")
}

func TestCopyAttesterSlashings(t *testing.T) {
	as := genAttesterSlashings(10)

	got := v1alpha1.CopyAttesterSlashings(as)
	if !reflect.DeepEqual(got, as) {
		t.Errorf("CopyAttesterSlashings() = %v, want %v", got, as)
	}
	assert.NotEmpty(t, got, "Copied attester slashings have empty fields")
}

func TestCopyIndexedAttestation(t *testing.T) {
	ia := genIndexedAttestation()

	got := v1alpha1.CopyIndexedAttestation(ia)
	if !reflect.DeepEqual(got, ia) {
		t.Errorf("CopyIndexedAttestation() = %v, want %v", got, ia)
	}
	assert.NotEmpty(t, got, "Copied indexed attestation has empty fields")
}

func TestCopyAttestations(t *testing.T) {
	atts := genAttestations(10)

	got := v1alpha1.CopyAttestations(atts)
	if !reflect.DeepEqual(got, atts) {
		t.Errorf("CopyAttestations() = %v, want %v", got, atts)
	}
	assert.NotEmpty(t, got, "Copied attestations have empty fields")
}

func TestCopyDeposits(t *testing.T) {
	d := genDeposits(10)

	got := v1alpha1.CopyDeposits(d)
	if !reflect.DeepEqual(got, d) {
		t.Errorf("CopyDeposits() = %v, want %v", got, d)
	}
	assert.NotEmpty(t, got, "Copied deposits have empty fields")
}

func TestCopyDeposit(t *testing.T) {
	d := genDeposit()

	got := v1alpha1.CopyDeposit(d)
	if !reflect.DeepEqual(got, d) {
		t.Errorf("CopyDeposit() = %v, want %v", got, d)
	}
	assert.NotEmpty(t, got, "Copied deposit has empty fields")
}

func TestCopyDepositData(t *testing.T) {
	dd := genDepositData()

	got := v1alpha1.CopyDepositData(dd)
	if !reflect.DeepEqual(got, dd) {
		t.Errorf("CopyDepositData() = %v, want %v", got, dd)
	}
	assert.NotEmpty(t, got, "Copied deposit data has empty fields")
}

func TestCopySignedVoluntaryExits(t *testing.T) {
	sv := genSignedVoluntaryExits(10)

	got := v1alpha1.CopySignedVoluntaryExits(sv)
	if !reflect.DeepEqual(got, sv) {
		t.Errorf("CopySignedVoluntaryExits() = %v, want %v", got, sv)
	}
	assert.NotEmpty(t, got, "Copied signed voluntary exits have empty fields")
}

func TestCopySignedVoluntaryExit(t *testing.T) {
	sv := genSignedVoluntaryExit()

	got := v1alpha1.CopySignedVoluntaryExit(sv)
	if !reflect.DeepEqual(got, sv) {
		t.Errorf("CopySignedVoluntaryExit() = %v, want %v", got, sv)
	}
	assert.NotEmpty(t, got, "Copied signed voluntary exit has empty fields")
}

func TestCopyValidator(t *testing.T) {
	v := genValidator()

	got := v1alpha1.CopyValidator(v)
	if !reflect.DeepEqual(got, v) {
		t.Errorf("CopyValidator() = %v, want %v", got, v)
	}
	assert.NotEmpty(t, got, "Copied validator has empty fields")
}

func TestCopySyncCommitteeMessage(t *testing.T) {
	scm := genSyncCommitteeMessage()

	got := v1alpha1.CopySyncCommitteeMessage(scm)
	if !reflect.DeepEqual(got, scm) {
		t.Errorf("CopySyncCommitteeMessage() = %v, want %v", got, scm)
	}
	assert.NotEmpty(t, got, "Copied sync committee message has empty fields")
}

func TestCopySyncCommitteeContribution(t *testing.T) {
	scc := genSyncCommitteeContribution()

	got := v1alpha1.CopySyncCommitteeContribution(scc)
	if !reflect.DeepEqual(got, scc) {
		t.Errorf("CopySyncCommitteeContribution() = %v, want %v", got, scc)
	}
	assert.NotEmpty(t, got, "Copied sync committee contribution has empty fields")
}

func TestCopySyncAggregate(t *testing.T) {
	sa := genSyncAggregate()

	got := v1alpha1.CopySyncAggregate(sa)
	if !reflect.DeepEqual(got, sa) {
		t.Errorf("CopySyncAggregate() = %v, want %v", got, sa)
	}
	assert.NotEmpty(t, got, "Copied sync aggregate has empty fields")
}

func TestCopyPendingAttestationSlice(t *testing.T) {
	tests := []struct {
		name  string
		input []*v1alpha1.PendingAttestation
	}{
		{
			name:  "nil",
			input: nil,
		},
		{
			name:  "empty",
			input: []*v1alpha1.PendingAttestation{},
		},
		{
			name: "correct copy",
			input: []*v1alpha1.PendingAttestation{
				genPendingAttestation(),
				genPendingAttestation(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v1alpha1.CopyPendingAttestationSlice(tt.input); !reflect.DeepEqual(got, tt.input) {
				t.Errorf("CopyPendingAttestationSlice() = %v, want %v", got, tt.input)
			}
		})
	}
}

func TestCopyPayloadHeader(t *testing.T) {
	p := genPayloadHeader()

	got := v1alpha1.CopyExecutionPayloadHeader(p)
	if !reflect.DeepEqual(got, p) {
		t.Errorf("CopyExecutionPayloadHeader() = %v, want %v", got, p)
	}
	assert.NotEmpty(t, got, "Copied execution payload header has empty fields")
}

func TestCopyPayloadHeaderCapella(t *testing.T) {
	p := genPayloadHeader()

	got := v1alpha1.CopyExecutionPayloadHeader(p)
	if !reflect.DeepEqual(got, p) {
		t.Errorf("TestCopyPayloadHeaderCapella() = %v, want %v", got, p)
	}
	assert.NotEmpty(t, got, "Copied execution payload header has empty fields")
}

func TestCopySignedBlindedBeaconBlock(t *testing.T) {
	sbb := genSignedBlindedBeaconBlock()

	got := v1alpha1.CopySignedBlindedBeaconBlock(sbb)
	if !reflect.DeepEqual(got, sbb) {
		t.Errorf("CopySignedBlindedBeaconBlock() = %v, want %v", got, sbb)
	}
	assert.NotEmpty(t, sbb, "Copied signed blinded beacon block Capella has empty fields")
}

func TestCopyBlindedBeaconBlock(t *testing.T) {
	b := genBlindedBeaconBlock()

	got := v1alpha1.CopyBlindedBeaconBlock(b)
	if !reflect.DeepEqual(got, b) {
		t.Errorf("CopyBlindedBeaconBlock() = %v, want %v", got, b)
	}
	assert.NotEmpty(t, b, "Copied blinded beacon block has empty fields")
}

func TestCopyBlindedBeaconBlockBody(t *testing.T) {
	bb := genBlindedBeaconBlockBody()

	got := v1alpha1.CopyBlindedBeaconBlockBodyCapella(bb)
	if !reflect.DeepEqual(got, bb) {
		t.Errorf("CopyBlindedBeaconBlockBodyCapella() = %v, want %v", got, bb)
	}
	assert.NotEmpty(t, bb, "Copied blinded beacon block body has empty fields")
}

func bytes(length int) []byte {
	b := make([]byte, length)
	for i := 0; i < length; i++ {
		b[i] = uint8(rand.Int31n(255) + 1)
	}
	return b
}

func TestCopyWithdrawals(t *testing.T) {
	ws := genWithdrawals(10)

	got := v1alpha1.CopyWithdrawalSlice(ws)
	if !reflect.DeepEqual(got, ws) {
		t.Errorf("TestCopyWithdrawals() = %v, want %v", got, ws)
	}
	assert.NotEmpty(t, got, "Copied withdrawals have empty fields")
}

func TestCopyWithdrawal(t *testing.T) {
	w := genWithdrawal()

	got := v1alpha1.CopyWithdrawal(w)
	if !reflect.DeepEqual(got, w) {
		t.Errorf("TestCopyWithdrawal() = %v, want %v", got, w)
	}
	assert.NotEmpty(t, got, "Copied withdrawal has empty fields")
}

func TestCopyDilithiumToExecutionChanges(t *testing.T) {
	changes := genDilithiumToExecutionChanges(10)

	got := v1alpha1.CopyDilithiumToExecutionChanges(changes)
	if !reflect.DeepEqual(got, changes) {
		t.Errorf("TestCopyDilithiumToExecutionChanges() = %v, want %v", got, changes)
	}
}

func TestCopyHistoricalSummaries(t *testing.T) {
	summaries := []*v1alpha1.HistoricalSummary{
		{BlockSummaryRoot: []byte("block summary root 0"), StateSummaryRoot: []byte("state summary root 0")},
		{BlockSummaryRoot: []byte("block summary root 1"), StateSummaryRoot: []byte("state summary root 1")},
	}

	got := v1alpha1.CopyHistoricalSummaries(summaries)
	if !reflect.DeepEqual(got, summaries) {
		t.Errorf("TestCopyHistoricalSummariesing() = %v, want %v", got, summaries)
	}
}

func genAttestation() *v1alpha1.Attestation {
	return &v1alpha1.Attestation{
		ParticipationBits: bytes(32),
		Data:              genAttData(),
		Signatures:        [][]byte{bytes(4595)},
	}
}

func genAttestations(num int) []*v1alpha1.Attestation {
	atts := make([]*v1alpha1.Attestation, num)
	for i := 0; i < num; i++ {
		atts[i] = genAttestation()
	}
	return atts
}

func genAttData() *v1alpha1.AttestationData {
	return &v1alpha1.AttestationData{
		Slot:            1,
		CommitteeIndex:  2,
		BeaconBlockRoot: bytes(32),
		Source:          genCheckpoint(),
		Target:          genCheckpoint(),
	}
}

func genCheckpoint() *v1alpha1.Checkpoint {
	return &v1alpha1.Checkpoint{
		Epoch: 1,
		Root:  bytes(32),
	}
}

func genZond1Data() *v1alpha1.Zond1Data {
	return &v1alpha1.Zond1Data{
		DepositRoot:  bytes(32),
		DepositCount: 4,
		BlockHash:    bytes(32),
	}
}

func genPendingAttestation() *v1alpha1.PendingAttestation {
	return &v1alpha1.PendingAttestation{
		ParticipationBits: bytes(32),
		Data:              genAttData(),
		InclusionDelay:    3,
		ProposerIndex:     5,
	}
}

func genSignedBeaconBlock() *v1alpha1.SignedBeaconBlock {
	return &v1alpha1.SignedBeaconBlock{
		Block:     genBeaconBlock(),
		Signature: bytes(4595),
	}
}

func genProposerSlashing() *v1alpha1.ProposerSlashing {
	return &v1alpha1.ProposerSlashing{
		Header_1: genSignedBeaconBlockHeader(),
		Header_2: genSignedBeaconBlockHeader(),
	}
}

func genProposerSlashings(num int) []*v1alpha1.ProposerSlashing {
	ps := make([]*v1alpha1.ProposerSlashing, num)
	for i := 0; i < num; i++ {
		ps[i] = genProposerSlashing()
	}
	return ps
}

func genAttesterSlashing() *v1alpha1.AttesterSlashing {
	return &v1alpha1.AttesterSlashing{
		Attestation_1: genIndexedAttestation(),
		Attestation_2: genIndexedAttestation(),
	}
}

func genIndexedAttestation() *v1alpha1.IndexedAttestation {
	return &v1alpha1.IndexedAttestation{
		AttestingIndices: []uint64{1, 2, 3},
		Data:             genAttData(),
		Signatures:       [][]byte{bytes(4595)},
	}
}

func genAttesterSlashings(num int) []*v1alpha1.AttesterSlashing {
	as := make([]*v1alpha1.AttesterSlashing, num)
	for i := 0; i < num; i++ {
		as[i] = genAttesterSlashing()
	}
	return as
}

func genBeaconBlockHeader() *v1alpha1.BeaconBlockHeader {
	return &v1alpha1.BeaconBlockHeader{
		Slot:          10,
		ProposerIndex: 15,
		ParentRoot:    bytes(32),
		StateRoot:     bytes(32),
		BodyRoot:      bytes(32),
	}
}

func genSignedBeaconBlockHeader() *v1alpha1.SignedBeaconBlockHeader {
	return &v1alpha1.SignedBeaconBlockHeader{
		Header:    genBeaconBlockHeader(),
		Signature: bytes(4595),
	}
}

func genDepositData() *v1alpha1.Deposit_Data {
	return &v1alpha1.Deposit_Data{
		PublicKey:             bytes(2592),
		WithdrawalCredentials: bytes(32),
		Amount:                20000,
		Signature:             bytes(4595),
	}
}

func genDeposit() *v1alpha1.Deposit {
	return &v1alpha1.Deposit{
		Data:  genDepositData(),
		Proof: [][]byte{bytes(32), bytes(32), bytes(32), bytes(32)},
	}
}

func genDeposits(num int) []*v1alpha1.Deposit {
	d := make([]*v1alpha1.Deposit, num)
	for i := 0; i < num; i++ {
		d[i] = genDeposit()
	}
	return d
}

func genVoluntaryExit() *v1alpha1.VoluntaryExit {
	return &v1alpha1.VoluntaryExit{
		Epoch:          5432,
		ValidatorIndex: 888888,
	}
}

func genSignedVoluntaryExit() *v1alpha1.SignedVoluntaryExit {
	return &v1alpha1.SignedVoluntaryExit{
		Exit:      genVoluntaryExit(),
		Signature: bytes(4595),
	}
}

func genSignedVoluntaryExits(num int) []*v1alpha1.SignedVoluntaryExit {
	sv := make([]*v1alpha1.SignedVoluntaryExit, num)
	for i := 0; i < num; i++ {
		sv[i] = genSignedVoluntaryExit()
	}
	return sv
}

func genValidator() *v1alpha1.Validator {
	return &v1alpha1.Validator{
		PublicKey:                  bytes(2592),
		WithdrawalCredentials:      bytes(32),
		EffectiveBalance:           12345,
		Slashed:                    true,
		ActivationEligibilityEpoch: 14322,
		ActivationEpoch:            14325,
		ExitEpoch:                  23425,
		WithdrawableEpoch:          30000,
	}
}

func genSyncCommitteeContribution() *v1alpha1.SyncCommitteeContribution {
	return &v1alpha1.SyncCommitteeContribution{
		Slot:              12333,
		BlockRoot:         bytes(32),
		SubcommitteeIndex: 4444,
		ParticipationBits: bytes(32),
		Signatures:        [][]byte{bytes(4595), bytes(4595)},
	}
}

func genSyncAggregate() *v1alpha1.SyncAggregate {
	return &v1alpha1.SyncAggregate{
		SyncCommitteeBits:       bytes(32),
		SyncCommitteeSignatures: [][]byte{bytes(4595)},
	}
}

func genBeaconBlockBody() *v1alpha1.BeaconBlockBody {
	return &v1alpha1.BeaconBlockBody{
		RandaoReveal:                bytes(4595),
		Zond1Data:                   genZond1Data(),
		Graffiti:                    bytes(32),
		ProposerSlashings:           genProposerSlashings(5),
		AttesterSlashings:           genAttesterSlashings(5),
		Attestations:                genAttestations(10),
		Deposits:                    genDeposits(5),
		VoluntaryExits:              genSignedVoluntaryExits(12),
		SyncAggregate:               genSyncAggregate(),
		ExecutionPayload:            genPayload(),
		DilithiumToExecutionChanges: genDilithiumToExecutionChanges(10),
	}
}

func genBeaconBlock() *v1alpha1.BeaconBlock {
	return &v1alpha1.BeaconBlock{
		Slot:          123455,
		ProposerIndex: 55433,
		ParentRoot:    bytes(32),
		StateRoot:     bytes(32),
		Body:          genBeaconBlockBody(),
	}
}

func genSignedBeaconBlockCapella() *v1alpha1.SignedBeaconBlock {
	return &v1alpha1.SignedBeaconBlock{
		Block:     genBeaconBlock(),
		Signature: bytes(4595),
	}
}

func genBlindedBeaconBlockBody() *v1alpha1.BlindedBeaconBlockBody {
	return &v1alpha1.BlindedBeaconBlockBody{
		RandaoReveal:                bytes(4595),
		Zond1Data:                   genZond1Data(),
		Graffiti:                    bytes(32),
		ProposerSlashings:           genProposerSlashings(5),
		AttesterSlashings:           genAttesterSlashings(5),
		Attestations:                genAttestations(10),
		Deposits:                    genDeposits(5),
		VoluntaryExits:              genSignedVoluntaryExits(12),
		SyncAggregate:               genSyncAggregate(),
		ExecutionPayloadHeader:      genPayloadHeader(),
		DilithiumToExecutionChanges: genDilithiumToExecutionChanges(10),
	}
}

func genBlindedBeaconBlock() *v1alpha1.BlindedBeaconBlock {
	return &v1alpha1.BlindedBeaconBlock{
		Slot:          123455,
		ProposerIndex: 55433,
		ParentRoot:    bytes(32),
		StateRoot:     bytes(32),
		Body:          genBlindedBeaconBlockBody(),
	}
}

func genSignedBlindedBeaconBlock() *v1alpha1.SignedBlindedBeaconBlock {
	return &v1alpha1.SignedBlindedBeaconBlock{
		Block:     genBlindedBeaconBlock(),
		Signature: bytes(4595),
	}
}

func genSyncCommitteeMessage() *v1alpha1.SyncCommitteeMessage {
	return &v1alpha1.SyncCommitteeMessage{
		Slot:           424555,
		BlockRoot:      bytes(32),
		ValidatorIndex: 5443,
		Signature:      bytes(4595),
	}
}

func genPayload() *enginev1.ExecutionPayload {
	return &enginev1.ExecutionPayload{
		ParentHash:    bytes(32),
		FeeRecipient:  bytes(20),
		StateRoot:     bytes(32),
		ReceiptsRoot:  bytes(32),
		LogsBloom:     bytes(256),
		PrevRandao:    bytes(32),
		BlockNumber:   1,
		GasLimit:      2,
		GasUsed:       3,
		Timestamp:     4,
		ExtraData:     bytes(32),
		BaseFeePerGas: bytes(32),
		BlockHash:     bytes(32),
		Transactions:  [][]byte{{'a'}, {'b'}, {'c'}},
		Withdrawals: []*enginev1.Withdrawal{
			{
				Index:          123,
				ValidatorIndex: 123,
				Address:        bytes(20),
				Amount:         123,
			},
			{
				Index:          124,
				ValidatorIndex: 456,
				Address:        bytes(20),
				Amount:         456,
			},
		},
	}
}

func genPayloadHeader() *enginev1.ExecutionPayloadHeader {
	return &enginev1.ExecutionPayloadHeader{
		ParentHash:       bytes(32),
		FeeRecipient:     bytes(20),
		StateRoot:        bytes(32),
		ReceiptsRoot:     bytes(32),
		LogsBloom:        bytes(256),
		PrevRandao:       bytes(32),
		BlockNumber:      1,
		GasLimit:         2,
		GasUsed:          3,
		Timestamp:        4,
		ExtraData:        bytes(32),
		BaseFeePerGas:    bytes(32),
		BlockHash:        bytes(32),
		TransactionsRoot: bytes(32),
		WithdrawalsRoot:  bytes(32),
	}
}

func genWithdrawals(num int) []*enginev1.Withdrawal {
	ws := make([]*enginev1.Withdrawal, num)
	for i := 0; i < num; i++ {
		ws[i] = genWithdrawal()
	}
	return ws
}

func genWithdrawal() *enginev1.Withdrawal {
	return &enginev1.Withdrawal{
		Index:          123456,
		ValidatorIndex: 654321,
		Address:        bytes(20),
		Amount:         55555,
	}
}

func genDilithiumToExecutionChanges(num int) []*v1alpha1.SignedDilithiumToExecutionChange {
	changes := make([]*v1alpha1.SignedDilithiumToExecutionChange, num)
	for i := 0; i < num; i++ {
		changes[i] = genDilithiumToExecutionChange()
	}
	return changes
}

func genDilithiumToExecutionChange() *v1alpha1.SignedDilithiumToExecutionChange {
	return &v1alpha1.SignedDilithiumToExecutionChange{
		Message: &v1alpha1.DilithiumToExecutionChange{
			ValidatorIndex:      123456,
			FromDilithiumPubkey: bytes(2592),
			ToExecutionAddress:  bytes(20),
		},
		Signature: bytes(4595),
	}
}