package params

const (
	// Every EL component has an offset that manages which port it is assigned. The miner always gets offset=0.
	ExecutionNodeComponentOffset = 0
	StaticFilesPath              = "/testing/endtoend/static-files/zond"
	keyFilename                  = "UTC--2024-01-04T08-08-35.961423000Z--Z2014d0daf1779e1319f4f93d3106d7918ba43b5a"
	baseELHost                   = "127.0.0.1"
	baseELScheme                 = "http"
	// DepositGasLimit is the gas limit used for all deposit transactions. The exact value probably isn't important
	// since these are the only transactions in the e2e run.
	DepositGasLimit = 4000000
	// SpamTxGasLimit is used for the spam transactions (to/from miner address)
	// which WaitForBlocks generates in order to advance the EL chain.
	SpamTxGasLimit = 21000
)
