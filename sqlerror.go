package gotds

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type SQLError struct {
	//The error number (numbers less than 20001 are reserved by Microsoft SQL Server).
	Number int32
	// The error state, used as a modifier to the error number.
	State byte
	// Class a.k.a. severity determines the severity of the error.
	// Values below 10 indicate informational messages
	Class byte
	// The error message itself
	Text string
	// The name of the server
	Server string
	// The name of the procedure that caused the error
	Procedure string
	// The line-number at which the error occured. 1-based
	// 0 means not applicable.
	Line int32
}

func (e SQLError) Error() string {
	return fmt.Sprintf("Msg %v, Level %v, State %v, Line %v\n%v", e.Number, e.Class, e.State, e.Line, e.Text)
}

func (c *Conn) makeError(raw []byte) SQLError {
	var sqlerr SQLError

	buf := bytes.NewBuffer(raw)
	token, err := buf.ReadByte()
	if err != nil {
		panic(err)
	}
	if token != 0xAA {
		panic("Invalid error token")
	}

	var length uint16
	err = binary.Read(buf, binary.LittleEndian, &length)
	if err != nil {
		panic(err)
	}

	err = binary.Read(buf, binary.LittleEndian, &sqlerr.Number)
	if err != nil {
		panic(err)
	}

	sqlerr.State, err = buf.ReadByte()
	if err != nil {
		panic(err)
	}

	sqlerr.Class, err = buf.ReadByte()
	if err != nil {
		panic(err)
	}

	sqlerr.Text = readUS_VarChar(buf)
	errLog.Printf(sqlerr.Text)
	sqlerr.Server = readB_VarChar(buf)
	sqlerr.Procedure = readB_VarChar(buf)

	if c.tdsVersion >= TDS72 {
		err = binary.Read(buf, binary.LittleEndian, &sqlerr.Line)
	} else {
		// TDS7.1 and earlier use a unsigned short instead of a long:
		var tempLine uint16
		err = binary.Read(buf, binary.LittleEndian, &sqlerr.Line)
		sqlerr.Line = int32(tempLine)
	}
	if err != nil {
		panic(err)
	}

	return sqlerr
}
