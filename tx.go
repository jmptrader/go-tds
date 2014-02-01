package gotds

type Tx struct {
	transactionHeader []byte
	stmtCount         int
}
