package gotds

import (
	"database/sql/driver"
	"errors"
	//"github.com/grovespaz/go-tds/safeconn"
	"bytes"
	"fmt"
	"io"
	"net"
	"time"
	"encoding/binary"
)

type PacketType byte

type ConnectionState int

const (
	headerSize = 8
)

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

const (
	ptySQLBatch    PacketType = 1
	ptyLegacyLogin PacketType = 2
	ptyRPC         PacketType = 3
	ptyTableResult PacketType = 4
	// Packettype 5 is unused
	ptyAttention PacketType = 6
	ptyBulkLoad  PacketType = 7
	// Packettypes 8-13 are unused
	ptyTransactionManagerRequest PacketType = 14
	// Packettype 15 is unused
	ptyLogin       PacketType = 16
	ptySSPIMessage PacketType = 17
	ptyPreLogin    PacketType = 18
)

type Conn struct {
	driver.Conn

	State    ConnectionState
	SubState SubState

	socket        io.ReadWriteCloser
	maxPacketSize int
	packetCount   byte

	cfg config
}

type config struct {
	user       string
	passwd     string
	net        string
	addr       string
	dbname     string
	params     map[string]string
	timeout    time.Duration
	verboseLog bool
}

func MakeConnection(cfg *config) (*Conn, error) {
	tcpConn, err := net.DialTimeout(cfg.net, cfg.addr, cfg.timeout)
	if err != nil {
		return nil, err
	}

	return MakeConnectionWithSocket(cfg, tcpConn)
}

func MakeConnectionWithSocket(cfg *config, socket io.ReadWriteCloser) (*Conn, error) {
	conn := &Conn{socket: socket, State: Initial, maxPacketSize: 1024, cfg: *cfg}

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
	//Parse results:
	conn.SubState = ParsingResponse
	// (Actually parse result here)
	if err != nil {
		conn.State = Error
		return nil, err
	}

	conn.SubState = Ready

	conn.State = PostLogin

	return conn, nil
}

func (c *Conn) Close() error {
	return c.socket.Close()
	//return nil
}

func (c *Conn) SendMessage(msgType PacketType, data []byte) (*[][]byte, error) {
	maxHeadlessPacketSize := c.maxPacketSize - headerSize

	//Split message into packets, send them all,
	i := 0

	for (len(data) - (i * maxHeadlessPacketSize)) > maxHeadlessPacketSize {
		v := (i * maxHeadlessPacketSize)
		view := data[v : v+maxHeadlessPacketSize]
		packet := makePacket(msgType, view, c.packetCount, false)
		i++
		(c.socket).Write(packet)
	}

	v := (i * maxHeadlessPacketSize)
	view := data[v:]
	packet := makePacket(msgType, view, c.packetCount, true)
	(c.socket).Write(packet)

	//collect all packets sent back.
	//Send response to caller
	EOM := false
	responses := make([][]byte, 0, 5)
	for !EOM {
		resultPacket := make([]byte, 1024, 1024)
		bytesRead, err := c.socket.Read(resultPacket)
		if err != nil {
			errLog.Println(err)
			return nil, err
		}

		if resultPacket[0] != 4 {
			//Server always returns type 4 in packet header
			err = errors.New("Incorrect data, was expecting 0x04.")
			errLog.Println(err)
			return nil, err
		}
		if resultPacket[1] == 1 {
			//Byte 1 in the packet header denotes status, 1 is EOM
			//This means that there are no more responses to be collected and we can give the response back to the caller
			EOM = true
		}
		if resultPacket[1] > 1 {
			//This should not happen in server->client communication.
			return nil, err
		}

		if c.cfg.verboseLog {
			errLog.Printf("Read %v bytes.\n", bytesRead)
			errLog.Printf("Result: %v\n", resultPacket[0:bytesRead])
		}
		responses = append(responses, resultPacket[8:bytesRead])
	}

	return &responses, nil
}

/*
Not actually used, but this is what a TDS packet-header woudl look like in a Go struct.
type packetHeader struct {
	pktType PacketType
	//status byte //filled outside
	//length uint16 //big endian, filled by datalength + 8
	//spid uint16 //unused, zeroed out
	//packetID byte //filled by connection
	//window byte //unused, zeroed out

	data *[]byte
}
*/

func makePacket(pktType PacketType, data []byte, packetID byte, lastRequest bool) []byte {
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

type tokenDefinition byte

const (
	zeroLength     tokenDefinition = 0x10 //0b00010000
	fixedLength    tokenDefinition = 0x30 //0b00110000
	variableLength tokenDefinition = 0x20 //0b00100000
	variableCount  tokenDefinition = 0x00 //0b00000000
)

type token struct {
	definition	tokenDefinition
	length		int
	data		[]byte
}

func parseTokenStream(data []byte) ([]token, error) {
	buf := bytes.NewBuffer(data)
	result := make([]token, 0, 10)

	nextToken, err := buf.ReadByte()
	for err == nil {
		errLog.Printf("Parsing token: %v\n", nextToken)
		newToken := token{}
		switch tokenDefinition(nextToken & 0x30) { //0x30 = 0b00110000
		case zeroLength:
			newToken.definition = zeroLength
			break
		case fixedLength:
			newToken.definition = fixedLength
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
			newToken.definition = variableLength
			var length uint16
			err := binary.Read(buf, binary.LittleEndian, &length)
			if err != nil {
				errLog.Println("binary.Read failed:", err)
				return nil, err
			}
		break;
		case variableCount:
			newToken.definition = variableCount
			panic("I haven't coded this part yet!")
		break;
		default:
			err = errors.New(fmt.Sprintf("Unknown Token Definition: %v", tokenDefinition(nextToken&0x30)))
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
	for _, tkn := range(tokens) {
		dataLength++
		tkn.length = len(tkn.data)
		dataLength += tkn.length
		
		if tkn.definition == fixedLength {
			validLength := (tkn.length == 1) || (tkn.length == 2) || (tkn.length == 4) || (tkn.length == 8)
			if !validLength {
				return nil, errors.New("Invalid length for fixedLength token")
			}
		}
		
		if tkn.definition == variableLength {
			dataLength += 2
		}
		
		
		if tkn.definition == variableCount {
			panic("I haven't coded this part yet!")
		}
	}
	
	result := make([]byte, 0, dataLength)
	buf := bytes.NewBuffer(result)
	for _, tkn := range(tokens) {
		if tkn.definition == fixedLength {
			var length byte
			
			switch len(tkn.data) {
			case 2:
				length = 0x4
			case 4:
				length = 0x8
			case 8:
				length = 0xc
			}
			
			_ = buf.WriteByte(byte(tkn.definition) & length) // Error is ALWAYS nil.
		} else {
			_ = buf.WriteByte(byte(tkn.definition)) // Error is ALWAYS nil.
		}
		
		if tkn.definition == variableLength {
			length := uint16(len(tkn.data))
			binary.Write(buf, binary.LittleEndian, length)
		}
		
		if tkn.definition == variableCount {
			panic("I haven't coded this part yet!")
		}
		
		buf.Write(tkn.data)
	}
	
	
	return result, nil
}
