package gotds

import (
	"database/sql/driver"
	"errors"
	//"github.com/grovespaz/go-tds/safeconn"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	driverName = "go-tds"
	//driverName           = "ODBC"
	driverVersion uint32 = 0x000001
	headerSize           = 8
)

const (
	TDS71  = 0x01000071
	TDS72  = 0x02000972
	TDS73  = 0x03000B73
	TDS73a = 0x03000a73 //Doesn't support NBCROW and Sparse Column sets
	TDS74  = 0x04000074
	/* Server does it differently, reversing the byte-order. That would be:
	serverTDS71 = 0x71000001
	serverTDS72 = 0x72090002
	serverTDS73 = 0x730B0003
	serverTDS74 = 0x74000004
	*/
)

type ConnectionState int

const (
	Initial ConnectionState = iota
	PreLogin
	Login
	PostLogin
	Error
	Disconnected
)

type SubState int

const (
	Ready SubState = iota
	RequestSent
	ParsingResponse
	NotificationSent
)

type packetType byte

const (
	ptySQLBatch    packetType = 1
	ptyLegacyLogin packetType = 2
	ptyRPC         packetType = 3
	ptyTableResult packetType = 4
	// packetType 5 is unused
	ptyAttention packetType = 6
	ptyBulkLoad  packetType = 7
	// packetTypes 8-13 are unused
	ptyTransactionManagerRequest packetType = 14
	// packetType 15 is unused
	ptyLogin       packetType = 16
	ptySSPIMessage packetType = 17
	ptyPreLogin    packetType = 18
)

type encryptionType byte

const (
	encryptOff          encryptionType = 0x00 //Encryption is available but off.
	encryptOn                          = 0x01 //Encryption is available and on.
	encryptNotSupported                = 0x02 //Encryption is not available.
	encryptRequired                    = 0x03 //Encryption is required.
)

type Conn struct {
	driver.Conn

	State    ConnectionState
	SubState SubState

	socket       io.ReadWriteCloser
	packetCount  byte
	tdsVersion   uint32
	clientPID    uint32
	connectionID uint32

	// 0 = X86, 1 = 68000
	byteOrder bool
	// 0 = ASCII, 1 = EBDDIC. This probably doesn't need to be configurable
	charType bool

	// 0 = IEEE_754, 1 = VAX, 2 = ND5000
	// Hardcoded for now
	//floatType bool

	// Do we need dump/load or BCP capabilities? 0 = ON, 1 = OFF for some reason
	dumpLoad bool
	// Do we want to be notified of changes in the database because of USE blahblahg statements.
	// Probably doesn't need to be configurable
	// 0 = No
	// 1 = Yes
	useDBWarnings bool

	// Do we want a warning if the language changes?
	setLang bool

	/*
		To the server, this determinse if the client is the ODBC driver.
		If true, it will set ANSI_DEFAULTS to ON, IMPLICIT_TRANSACTIONS to OFF, TEXTSIZE to 0x7FFFFFFF
		(2GB) (TDS 7.2 and earlier), TEXTSIZE to infinite (introduced in TDS 7.3), and
		ROWCOUNT to infinite.
	*/
	odbc bool

	useOLEDB bool //Since TDS 7.2

	cfg config
}

type config struct {
	user          string
	password      string
	net           string
	addr          string
	dbname        string
	params        map[string]string
	timeout       time.Duration
	verboseLog    bool
	maxPacketSize uint32
	appname       string //Optional: name of the application.
	attachDB      string //Optional: filename of database to attach upon connecting.

	encryption             encryptionType
	trustServerCertificate bool

	// If set to true, we can't connect if we can't change to the initial DB specified.
	failIfNoDB bool
	// Same as above, but with language
	failIfNoLanguage bool
	readOnly         bool //Since TDS 7.4, ignored under that

	changePass bool //Since TDS 7.2
	newPass    string

	userInstance bool //Since TDS 7.2

	preferredLanguage string //?Really?

	integratedSecurity bool

	// Type of SQL we are going to send to the server.
	// 0 = DFLT (I assume default?), 1 = T-SQL
	// I assume everyone will use T-SQL but what the hey...
	sqlType bool //Documented to be 4 bits but only 1 bit is used at the moment to distinguish between default (T-SQL) and explicit T-SQL... Go figure!

	timezone int32  // TODO: Figure out format, best guess: minutes difference between UTC and local time. UTC = local time + timezone
	lcid     uint32 //Microsoft Locale Identifier. 1033 (0x0409) == US English
}

// MakeConnection initiates a TCP connection with the specified configuration.
func MakeConnection(cfg *config) (*Conn, error) {
	tcpConn, err := net.DialTimeout(cfg.net, cfg.addr, cfg.timeout)
	if err != nil {
		return nil, err
	}

	return MakeConnectionWithSocket(cfg, tcpConn)
}

// MakeConnectionWithSocket initiates a connection using the specified ReadWriteCloser as an underlying socket.
// This allows for the TDS connections to take place over a protocol other than TCP, which the specs allow for.
func MakeConnectionWithSocket(cfg *config, socket io.ReadWriteCloser) (*Conn, error) {
	conn := &Conn{socket: socket, State: Initial, cfg: *cfg, tdsVersion: TDS73}

	//This seems reasonable?:
	conn.useDBWarnings = true
	conn.setLang = true
	conn.odbc = true

	//Some other defaults for now:
	conn.cfg.timezone = 0x000001e0
	conn.cfg.lcid = 0x00000409

	conn.State = PreLogin
	conn.SubState = RequestSent

	//Send pre-login:
	response, err := conn.sendPreLogin()
	if err != nil {
		return nil, err
	}

	//Parse results:
	conn.SubState = ParsingResponse
	// Actually parse result here
	// Right now it's not super-important to parse this, I'd rather get other parts functioning now.
	// Eventually it will be more useful to determine the server version, encryption required, etc.
	// So lets pretend to do something with it:
	_ = response
	if err != nil {
		conn.State = Error
		return nil, err
	}

	conn.SubState = Ready

	conn.State = Login
	conn.SubState = RequestSent
	//Send Login packet (eventually: negotiate encryption and stuff)
	loginResult, err := conn.login()
	if err != nil {
		conn.State = Error
		return nil, err
	}

	//Parse results:
	_ = loginResult
	conn.SubState = ParsingResponse
	// (Actually parse result here)
	if err != nil {
		conn.State = Error
		return nil, err
	}

	// For now we assume that, if no errors occured, we're good to go!
	conn.SubState = Ready

	conn.State = PostLogin

	return conn, nil
}

// Immediately closes the socket.
func (c *Conn) Close() error {
	return c.socket.Close()
	//return nil
}

// sendMessage sends the supplied data to the server, wrapped in the proper headers and packet(s)
// You probably shouldn't use this directly.
func (c *Conn) sendMessage(msgType packetType, data []byte) (*[][]byte, *[]SQLError, error) {

	maxHeadlessPacketSize := int(c.cfg.maxPacketSize - headerSize)

	//Split message into packets, send them all,
	i := 0

	for (len(data) - (i * maxHeadlessPacketSize)) > maxHeadlessPacketSize {
		v := (i * maxHeadlessPacketSize)
		view := data[v : v+maxHeadlessPacketSize]
		packet := makePacket(msgType, view, c.packetCount, false)
		c.packetCount++
		i++
		if c.cfg.verboseLog {
			errLog.Printf("Writing: %v", packet)
		}
		(c.socket).Write(packet)
	}

	v := (i * maxHeadlessPacketSize)
	view := data[v:]

	//errLog.Printf("Packet count: %X", c.packetCount)
	packet := makePacket(msgType, view, c.packetCount, true)
	c.packetCount++
	//errLog.Printf("Packet count: %X", c.packetCount)

	if c.cfg.verboseLog {
		errLog.Printf("Writing: % X", packet)
	}

	(c.socket).Write(packet)

	return c.readMessage()
}

func (c *Conn) readMessage() (*[][]byte, *[]SQLError, error) {
	//collect all packets sent back.
	//Send response to caller
	EOM := false
	responses := make([][]byte, 0, 5)
	SQLErrors := make([]SQLError, 0, 5)
	for !EOM {
		resultPacket := make([]byte, 1024, 1024)
		bytesRead, err := c.socket.Read(resultPacket)
		if err != nil {
			errLog.Println(err)
			return nil, nil, err
		}

		if c.cfg.verboseLog {
			errLog.Printf("Read %v bytes.\n", bytesRead)
			errLog.Printf("Result: % x\n", resultPacket[0:bytesRead])
		}

		if resultPacket[0] != byte(ptyTableResult) {
			//Server always returns type 4 in packet header
			err = errors.New("Incorrect data, was expecting 0x04.")
			errLog.Println(err)
			return nil, nil, err
		}
		if resultPacket[1] == 1 {
			//Byte 1 in the packet header denotes status, 1 is EOM
			//This means that there are no more responses to be collected and we can give the response back to the caller
			EOM = true
		}
		if resultPacket[1] > 1 {
			//This should not happen in server->client communication.
			return nil, nil, err
		}

		switch resultPacket[8] {
		case 0xAA: //Error, TODO(gv): Do something with these...
			if c.cfg.verboseLog {
				errLog.Printf("Received error.\n")
			}
			SQLErrors = append(SQLErrors, SQLError{Text: "Placeholder-error"})
		default:
			if c.cfg.verboseLog {
				errLog.Printf("Received non-error.\n")
			}
			responses = append(responses, resultPacket[8:bytesRead])
		}
	}

	return &responses, &SQLErrors, nil
}

/*
Not actually used, but this is what a TDS packet-header would look like in a Go struct.
type packetHeader struct {
	pktType packetType
	//status byte //filled outside
	//length uint16 //big endian, filled by datalength + 8
	//spid uint16 //unused, zeroed out
	//packetID byte //filled by connection
	//window byte //unused, zeroed out

	data *[]byte
}
*/

func makePacket(pktType packetType, data []byte, packetID byte, lastRequest bool) []byte {
	//headerSize = 8
	result := make([]byte, headerSize, headerSize+len(data))
	result[0] = byte(pktType)

	if lastRequest {
		result[1] = 1
	} else {
		result[1] = 0
	}

	length := uint16(8 + len(data))
	result[2] = byte(length / 256)
	result[3] = byte(length % 256)

	//result[4] = 0 //SP...
	//result[5] = 0 //...ID
	result[6] = packetID
	//packetID++
	//result[7] = 0 //Window

	result = append(result, data...)

	return result
}

type tokenLengthDefinition byte

const (
	zeroLength     tokenLengthDefinition = 0x10 //0b00010000
	fixedLength    tokenLengthDefinition = 0x30 //0b00110000
	variableLength tokenLengthDefinition = 0x20 //0b00100000
	variableCount  tokenLengthDefinition = 0x00 //0b00000000
)

type tokenDefinition byte

const (
	altMetaData   tokenDefinition = 0x88
	altRow        tokenDefinition = 0xD3
	colInfo       tokenDefinition = 0xA5
	colMetaData   tokenDefinition = 0x81
	done          tokenDefinition = 0xFD
	doneInProc    tokenDefinition = 0xFF
	doneProc      tokenDefinition = 0xFE
	envChange     tokenDefinition = 0xE3
	errorToken    tokenDefinition = 0xAA // of course 'error' is a reserved string :)
	featureExtAck tokenDefinition = 0xAE
	info          tokenDefinition = 0xAB
	loginAck      tokenDefinition = 0xAD
	nbcRow        tokenDefinition = 0xD2
	offset        tokenDefinition = 0x78 //Removed in TDS 7.2
	order         tokenDefinition = 0xA9
	returnStatus  tokenDefinition = 0x79
	returnValue   tokenDefinition = 0xAC
	row           tokenDefinition = 0xD1
	sessionState  tokenDefinition = 0xE4
	sspi          tokenDefinition = 0xED
	tabName       tokenDefinition = 0xA4
	tvpRow        tokenDefinition = 0x01
)

type token struct {
	definition tokenDefinition
	length     int
	data       []byte
}

func parseTokenStream(data []byte) ([]token, error) {
	buf := bytes.NewBuffer(data)
	result := make([]token, 0, 10)

	nextToken, err := buf.ReadByte()
	for err == nil {
		errLog.Printf("Parsing token: %v\n", nextToken)
		newToken := token{definition: tokenDefinition(nextToken)}
		switch tokenLengthDefinition(nextToken & 0x30) { //0x30 = 0b00110000, these two bits decide how to parse the token
		case zeroLength:
			//newToken.definition = zeroLength
			break
		case fixedLength:
			errLog.Println("Fixedlength")
			//newToken.definition = fixedLength
			byteCount := 1
			switch nextToken & 0xc { //0xc = 0b1100
			case 0x4:
				byteCount = 2
			case 0x8:
				byteCount = 4
			case 0xc:
				byteCount = 8
			}
			tokenData := buf.Next(byteCount)
			newToken.data = tokenData
			newToken.length = byteCount
			break
		case variableLength:
			errLog.Println("variableLength")
			//newToken.definition = variableLength
			var length uint16
			err := binary.Read(buf, binary.BigEndian, &length)
			if err != nil {
				errLog.Println("binary.Read failed:", err)
				return nil, err
			}
			newToken.length = int(length)
			newToken.data = buf.Next(newToken.length)
			break
		case variableCount:
			//newToken.definition = variableCount
			panic("I haven't coded this part yet!")
			break
		default:
			err = errors.New(fmt.Sprintf("Unknown Token-length Definition: %v", tokenLengthDefinition(nextToken&0x30)))
			errLog.Println(err)
			return nil, err
			break
		}
		errLog.Printf("Parsed token: %v\n", nextToken)
		result = append(result, newToken)

		nextToken, err = buf.ReadByte()
	}
	if err != io.EOF {
		return nil, err
	}
	return result, nil
}

func makeTokenStream(tokens []token) ([]byte, error) {
	//PerformanceTodo: Check if determining the length first to make one proper allocation instead of growing a few times actually helps.
	//					Also compare performance to just allocating a sufficiently large buffer.
	var dataLength int
	for _, tkn := range tokens {
		dataLength++
		tkn.length = len(tkn.data)
		dataLength += tkn.length

		tknLengthDef := tokenLengthDefinition(tkn.definition)

		if tknLengthDef == fixedLength {
			validLength := (tkn.length == 1) || (tkn.length == 2) || (tkn.length == 4) || (tkn.length == 8)
			if !validLength {
				return nil, errors.New("Invalid length for fixedLength token")
			}
		}

		if tknLengthDef == variableLength {
			dataLength += 2
		}

		if tknLengthDef == variableCount {
			panic("I haven't coded this part yet!")
		}
	}

	result := make([]byte, 0, dataLength)
	buf := bytes.NewBuffer(result)
	for _, tkn := range tokens {
		tknLengthDef := tokenLengthDefinition(tkn.definition)

		if tknLengthDef == fixedLength {
			var length byte

			switch len(tkn.data) {
			case 2:
				length = 0x4
			case 4:
				length = 0x8
			case 8:
				length = 0xc
			}

			buf.WriteByte(byte(tkn.definition) | length)
		} else {
			buf.WriteByte(byte(tkn.definition))
		}

		if tknLengthDef == variableLength {
			length := uint16(len(tkn.data))
			binary.Write(buf, binary.BigEndian, length) //PerformanceTodo: Check the cost of this versus manual mod-ing / shifting
		}

		if tknLengthDef == variableCount {
			panic("I haven't coded this part yet!")
		}

		buf.Write(tkn.data)
	}

	return buf.Bytes(), nil
}
