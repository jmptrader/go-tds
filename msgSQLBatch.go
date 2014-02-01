package gotds

import (
	"bytes"
	"database/sql/driver"
)

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

	return c.parseResult((*queryResultData)[0])
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
