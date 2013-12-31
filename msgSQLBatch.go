package gotds

import (
	"bytes"
	"database/sql/driver"
)

// Stmt is a shim at the moment until I implement proper parameter handling
type Stmt struct {
	c         *Conn
	statement string
}

func (s Stmt) Close() error {
	// Nothing to do
	return nil
}

func (s Stmt) NumInput() int {
	//At the moment I don't support placeholders/parameters
	return -1
}

func (s Stmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.c.Exec(s.statement, args)
}

func (s Stmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.c.Query(s.statement, args)
}

func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	return Stmt{statement: query}, nil
}

func (c *Conn) Exec(query string, args []driver.Value) (driver.Result, error) {
	if c.cfg.verboseLog {
		errLog.Printf("Executing query: %v", query)
	}

	queryPacket, err := c.makeSQLBatchPacket(query, args)
	if err != nil {
		return nil, err
	}

	queryResultData, sqlerr, err := c.sendMessage(ptySQLBatch, queryPacket)

	if err != nil {
		return nil, err
	}

	if len(*sqlerr) > 0 {
		// For now:
		return nil, (*sqlerr)[0]
	}

	if c.cfg.verboseLog {
		errLog.Printf("Request: % x\n", queryPacket)
		errLog.Printf("Response: % x\n", queryResultData)
	}

	return nil, nil
}

func (c *Conn) Query(query string, args []driver.Value) (driver.Rows, error) {
	if c.cfg.verboseLog {
		errLog.Printf("Executing query: %v", query)
	}

	queryPacket, err := c.makeSQLBatchPacket(query, args)
	if err != nil {
		return nil, err
	}

	//queryPacket = []byte{0x16, 0x00, 0x00, 0x00, 0x12, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x73, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x65, 0x00, 0x63, 0x00, 0x74, 0x00, 0x20, 0x00, 0x27, 0x00, 0x66, 0x00, 0x6F, 0x00, 0x6F, 0x00, 0x27, 0x00, 0x20, 0x00, 0x61, 0x00, 0x73, 0x00, 0x20, 0x00, 0x27, 0x00, 0x62, 0x00, 0x61, 0x00, 0x72, 0x00, 0x27, 0x00, 0x0A, 0x00, 0x20, 0x00, 0x20, 0x00, 0x20, 0x00, 0x20, 0x00, 0x20, 0x00, 0x20, 0x00, 0x20, 0x00, 0x20, 0x00}

	queryResultData, sqlerr, err := c.sendMessage(ptySQLBatch, queryPacket)

	if err != nil {
		return nil, err
	}

	if len(*sqlerr) > 0 {
		// For now:
		return nil, (*sqlerr)[0]
	}

	if c.cfg.verboseLog {
		errLog.Printf("Request: % x\n", queryPacket)
		errLog.Printf("Response: % x\n", queryResultData)
	}

	return nil, nil
}

func (c *Conn) makeSQLBatchPacket(query string, args []driver.Value) ([]byte, error) {
	b := new(bytes.Buffer)
	b.Grow(0) // TODO(gv): Fill in the least needed amount here

	// TODO(gv): Support proper transactions here
	transactionHeader := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	outstandingRequests := 1

	writeCommonHeader(b, transactionHeader, outstandingRequests)

	escapedQuery, err := escapeParameters(query, args, c.cfg.placeholder)
	if err != nil {
		return nil, err
	}
	writeUTF16String(b, escapedQuery)

	return b.Bytes(), nil
}
