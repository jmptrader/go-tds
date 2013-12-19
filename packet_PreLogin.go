package gotds

import (
	"errors"
)

type pl_option_token byte

const (
	VERSION    pl_option_token = 0x00
	ENCRYPTION pl_option_token = 0x01
	INSTOPT                    = 0x02
	THREADID                   = 0x03
	MARS                       = 0x04
	TRACEID                    = 0x05
	TERMINATOR                 = 0xFF
)

func (c *Conn) sendPreLogin() ([]byte, error) {
	preLoginPacket := makePreLoginPacket(0, encryptNotSupported, "", 0, false, [...]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	preLoginResult, sqlerr, err := c.sendMessage(ptyPreLogin, preLoginPacket)

	if err != nil {
		return nil, err
	}

	if len(*preLoginResult) == 0 {
		return nil, errors.New("No preLogin response")
	}

	if len(*preLoginResult) > 1 {
		return nil, errors.New("More than 1 result in the preLogin response")
	}

	if len(*sqlerr) > 0 {
		return nil, (*sqlerr)[0]
	}

	preLoginResultData := (*preLoginResult)[0] //[8:]

	if c.cfg.verboseLog {
		errLog.Printf("Request: %v\n", preLoginPacket)
		errLog.Printf("Response: %v\n", preLoginResultData)
	}
	return preLoginResultData, nil
}

type preLoginOptions []preLoginOption
type preLoginOption struct {
	option     pl_option_token
	data       []byte
	dataLength uint16 //TODO: Find out max length (USHORT, big endian), fill in proper go datatype
	offset     uint16 //TODO: Find out max length (USHORT, big endian), fill in proper go datatype
}

func makePreLoginPacket(version int, encryption encryptionType, instanceName string, ThreadID int, mars bool, traceID [20]byte) []byte {
	// Memory allocation might be better controlled with a pool
	packetData := make([]byte, 0, 100)
	preLoginData := make([]byte, 0, 100)

	options := preLoginOptions{
		preLoginOption{option: VERSION, data: []byte{0x09, 0x0, 0x0, 0x0, 0x0, 0x0}},
		preLoginOption{option: ENCRYPTION, data: []byte{byte(encryption)}},
		preLoginOption{option: MARS, data: []byte{0}},
		//preLoginOption{option: TERMINATOR, data: []byte()},
	}

	startingOffset := (len(options) * (1 + 2 + 2)) + 1 // 1 for pl_option_token, 2 twice for PL_OFFSET and PL_OPTION_LENGTH. +1 for terminator

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
