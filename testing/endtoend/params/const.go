package params

const (
	// Every EL component has an offset that manages which port it is assigned. The miner always gets offset=0.
	ExecutionNodeComponentOffset = 0
	StaticFilesPath              = "/testing/endtoend/static-files/qrl"
	keyFilename                  = "UTC--2024-01-04T08-08-35.961423000Z--Qaf84bc06703edfc371a0177ac8b482622d5ad242"
	baseELHost                   = "127.0.0.1"
	baseELScheme                 = "http"
	// DepositGasLimit is the gas limit used for all deposit transactions. The exact value probably isn't important
	// since these are the only transactions in the e2e run.
	DepositGasLimit = 4000000
	// SpamTxGasLimit is used for the spam transactions (to/from miner address)
	// which WaitForBlocks generates in order to advance the EL chain.
	SpamTxGasLimit = 21000
)
