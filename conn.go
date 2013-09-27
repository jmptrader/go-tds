package gotds

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"github.com/grovespaz/go-tds/safeconn"
	"io"
	"net"
	"time"
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

const (
	ptySQLBatch    safeconn.PacketType = 1
	ptyLegacyLogin safeconn.PacketType = 2
	ptyRPC         safeconn.PacketType = 3
	ptyTableResult safeconn.PacketType = 4
	// Packettype 5 is unused
	ptyAttention safeconn.PacketType = 6
	ptyBulkLoad  safeconn.PacketType = 7
	// Packettypes 8-13 are unused
	ptyTransactionManagerRequest safeconn.PacketType = 14
	// Packettype 15 is unused
	ptyLogin       safeconn.PacketType = 16
	ptySSPIMessage safeconn.PacketType = 17
	ptyPreLogin    safeconn.PacketType = 18
)

type Conn struct {
	driver.Conn
	socket *safeconn.SafeConn

	State    ConnectionState
	SubState SubState
}

func MakeConnection(name string) (*Conn, error) {
	tcpConn, err := net.DialTimeout("tcp", "slu.is:49286", time.Second*10)
	if err != nil {
		return nil, err
	}

	return MakeConnectionWithSocket(name, tcpConn)
}

func MakeConnectionWithSocket(name string, tcpConn io.ReadWriteCloser) (*Conn, error) {
	safeConn := safeconn.MakeSafeConn(tcpConn, 1024)
	conn := &Conn{socket: safeConn, State: Initial}

	conn.State = PreLogin
	conn.SubState = RequestSent

	preLoginPacket := makePreLoginPacket(0, ENCRYPT_OFF, "", 0, false, [...]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	chanLogin := conn.socket.SendMessage(&safeconn.Message{MsgType: ptyPreLogin, Data: preLoginPacket})

	preLoginResult := <-chanLogin

	if len(*preLoginResult.Responses) == 0 {
		return nil, errors.New("No preLogin response")
	}

	if len(*preLoginResult.Responses) > 1 {
		return nil, errors.New("More than 1 result in the preLogin response")
	}

	preLoginResultData := (*preLoginResult.Responses)[0][8:]

	fmt.Println("Request:")
	fmt.Println(preLoginPacket)

	fmt.Println("Response:")
	fmt.Println(preLoginResultData)

	/*
		for i, b := range(*loginResult.Responses) {
			fmt.Println("-------------------------------")
			fmt.Println(i)
			fmt.Println(b)
		}
		fmt.Println("-------------------------------")
	*/

	//Send PreLogin packet
	//Parse results:
	conn.SubState = ParsingResponse
	//if error, set state, return error.
	//if success:
	conn.SubState = Ready

	conn.State = Login
	conn.SubState = RequestSent
	//Send Login packet (eventually: negotiate encryption and stuff)
	//Parse results:
	conn.SubState = ParsingResponse
	//if error, set state, return error.
	//if success:
	conn.SubState = Ready

	conn.State = PostLogin

	return conn, nil
}

func (c *Conn) Close() error {
	//return c.socket.tcpConn.Close()
	return nil
}

/*
func (db *DB) Query(query string, args ...interface{}) (*Rows, error) {

}
*/

type PL_OPTION_TOKEN byte

const (
	VERSION    PL_OPTION_TOKEN = 0x00
	ENCRYPTION PL_OPTION_TOKEN = 0x01
	INSTOPT                    = 0x02
	THREADID                   = 0x03
	MARS                       = 0x04
	TRACEID                    = 0x05
	TERMINATOR                 = 0xFF
)

type B_FENCRYPTION byte

const (
	ENCRYPT_OFF     B_FENCRYPTION = 0x00 //Encryption is available but off.
	ENCRYPT_ON                    = 0x01 //Encryption is available and on.
	ENCRYPT_NOT_SUP               = 0x02 //Encryption is not available.
	ENCRYPT_REQ                   = 0x03 //Encryption is required.
)

type preLoginOptions []preLoginOption
type preLoginOption struct {
	option     PL_OPTION_TOKEN
	data       []byte
	dataLength uint16 //TODO: Find out max length (USHORT, big endian), fill in proper go datatype
	offset     uint16 //TODO: Find out max length (USHORT, big endian), fill in proper go datatype
}

func makePreLoginPacket(version int, encryption B_FENCRYPTION, instanceName string, ThreadID int, mars bool, traceID [20]byte) []byte {
	// Memory allocation might be better controlled with a pool
	packetData := make([]byte, 0, 100)
	preLoginData := make([]byte, 0, 100)

	options := preLoginOptions{
		preLoginOption{option: VERSION, data: []byte{0x09, 0x0, 0x0, 0x0, 0x0, 0x0}},
		preLoginOption{option: ENCRYPTION, data: []byte{byte(encryption)}},
		preLoginOption{option: MARS, data: []byte{0}},
		//preLoginOption{option: TERMINATOR, data: []byte()},
	}

	startingOffset := (len(options) * (1 + 2 + 2)) + 1 // 1 for PL_OPTION_TOKEN, 2 twice for PL_OFFSET and PL_OPTION_LENGTH. +1 for terminator

	for _, plo := range options {
		length := len(plo.data)
		preLoginData = append(preLoginData, plo.data...)
		packetData = append(packetData, byte(plo.option), byte(startingOffset/256), byte(startingOffset%256), byte(length/256), byte(length%256))
		startingOffset += length
	}
	packetData = append(packetData, byte(TERMINATOR))
	packetData = append(packetData, preLoginData...)
	return packetData
}
