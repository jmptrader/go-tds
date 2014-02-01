package gotds

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
)

var (
	ErrInvalidData = errors.New("Invalid data found")
)

type columnType byte

type columnInfo struct {
	columnType columnType
}

const (
	// Fixed-length:
	// 0-length:
	NULLTYPE columnType = 0x1F // Null
	// 1-length:
	INT1TYPE columnType = 0x30 // TinyInt
	BITTYPE  columnType = 0x32 // Bit
	// 2-length:
	INT2TYPE columnType = 0x34 // SmallInt
	// 4-length:
	INT4TYPE     columnType = 0x38 // Int
	DATETIM4TYPE columnType = 0x3A // SmallDateTime
	FLT4TYPE     columnType = 0x3B // Real
	MONEY4TYPE   columnType = 0x7A // SmallMoney
	// 8-length:
	MONEYTYPE    columnType = 0x3C // Money
	DATETIMETYPE columnType = 0x3D // DateTime
	FLT8TYPE     columnType = 0x3E // Float
	INT8TYPE     columnType = 0x7F // BigInt

)

// Unused at the moment:
//var fixedLengthTypes = []columnType{NULLTYPE, INT1TYPE, BITTYPE, INT2TYPE, INT4TYPE, DATETIM4TYPE, FLT4TYPE, MONEY4TYPE, MONEYTYPE, DATETIMETYPE, FLT8TYPE, INT8TYPE}

type Rows struct {
	columnNames []string
	columnTypes []columnInfo
	buf         bytes.Buffer
}

func (r Rows) Columns() []string {
	return r.columnNames
}

func (r Rows) Close() error {
	return nil
}

func (r Rows) Next(dest []driver.Value) error {
	if r.buf.Len() == 0 {
		return io.EOF
	}
	for i, m := range r.columnTypes {
		var v interface{}
		switch m.columnType {
		case INT4TYPE:
			d := make([]byte, 0, 4)
			n, err := r.buf.Read(d)
			if err != nil {
				return nil
			}
			if n != 4 {
				return ErrInvalidData
			}
			v = int64(d[0]) & int64(d[1]<<8) & int64(d[2]<<16) & int64(d[3]<<24)
		default:
			return ErrInvalidData

		}
		dest[i] = v
	}
	return nil
}

func (c *Conn) parseResult(raw []byte) (Rows, error) {
	var rows Rows
	if raw[1] == 0xff && raw[2] == 0xff {
		panic("Got noMetaData in COLMETADATA, should not happen")
	}
	buf := bytes.NewBuffer(raw)
	fieldcount, err := buf.ReadByte()
	if err != nil {
		return rows, err
	}
	buf.ReadByte()
	for i := byte(0); i < fieldcount; i++ {
		// UserType (we ignore this for now)
		// Will always be 0x0000 except for TIMESTAMP (0x0050) and alias types (greater than 0x00FF).
		if c.tdsVersion >= TDS72 {
			// ULONG (uint32) in TDS72 and higher
			_ = buf.Next(4)
		} else {
			// USHORT (uint16) in TDS71 and lower
			_ = buf.Next(2)
		}

		// Flags, also ignorable for now
		_ = buf.Next(2)

		// Type info:
		info, err := parseColumnType(buf)
		if err != nil {
			return rows, err
		}

		// If  text, ntext and image, table name:
		if false {
			//Commented because not yet used:
			//tablename := readB_VarChar(buf)
		}

		// Column name
		columnName := readB_VarChar(buf)

		rows.columnNames = append(rows.columnNames, columnName)
		rows.columnTypes = append(rows.columnTypes, info)
	}
	return rows, nil
}

func parseColumnType(buf *bytes.Buffer) (columnInfo, error) {
	var result columnInfo
	// Loose byte?
	//fmt.Printf("% X \n", buf.Bytes())
	//_, _ = buf.ReadByte()
	// _, _ = buf.ReadByte()
	fmt.Printf("% X \n", buf.Bytes())
	b, err := buf.ReadByte()
	fmt.Printf("% X \n", buf.Bytes())
	if err != nil {
		return result, err
	}
	ctype := columnType(b)
	result.columnType = ctype
	switch ctype {
	case NULLTYPE:
		fallthrough
	case INT1TYPE:
		fallthrough
	case BITTYPE:
		fallthrough
	case INT2TYPE:
		fallthrough
	case INT4TYPE:
		fallthrough
	case DATETIM4TYPE:
		fallthrough
	case FLT4TYPE:
		fallthrough
	case MONEY4TYPE:
		fallthrough
	case MONEYTYPE:
		fallthrough
	case DATETIMETYPE:
		fallthrough
	case FLT8TYPE:
		fallthrough
	case INT8TYPE:
		// Nothing to do for these, length is contained within type
	default:
		panic(fmt.Sprintf("unknown column type: %x", b))
	}
	return result, nil
}
