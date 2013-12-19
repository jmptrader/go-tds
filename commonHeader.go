package gotds

import (
	//"bytes"
	"encoding/binary"
	"io"
)

// The following function builds a header that is common to multiple messages.
// Although the TDS protocol supports three types of headers within this header, go-tds currently only supports one (TransactionHeader).
// This is very hardcoded at the moment but can be extended later.
// The current header contains the bare minimum needed, which is a transaction header.
func writeCommonHeader(buf io.Writer, TransactionDescriptor []byte, OutstandingRequests int) {
	header := []byte{
		0x16, 0, 0, 0, //TotalHeaderLength
		0x12, 0, 0, 0, //TransactionHeaderLength
		2, 0} //TransactionHeaderType
	_, err := buf.Write(header)
	if err != nil {
		panic(err)
	}

	_, err = buf.Write(TransactionDescriptor) // 00 00 00 00 00 00 00 01
	if err != nil {
		panic(err)
	}

	binary.Write(buf, binary.BigEndian, uint32(OutstandingRequests)) // 00 00 00 00
}
