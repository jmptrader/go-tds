package safeconn

import (
	"fmt"
	"io"
)

type PacketType byte

type Message struct {
	MsgType PacketType
	Data    []byte
}

type wrappedMessage struct {
	Message
	responseChannel chan (*Response)
}

type Response struct {
	Responses *[][]byte
}

type SafeConn struct {
	tcpConn   io.ReadWriteCloser
	sendQueue chan (*wrappedMessage)

	/* Low-level packet variables: */
	maxPacketSize int
}

func MakeSafeConn(pipe io.ReadWriteCloser, maxPacketSize int) *SafeConn {
	result := &SafeConn{tcpConn: pipe, sendQueue: make(chan *wrappedMessage), maxPacketSize: maxPacketSize}
	go result.sendLoop()
	return result
}

const (
	headerSize = 8
)

func (c *SafeConn) sendLoop() {
	var packetCount byte
	maxHeadlessPacketSize := c.maxPacketSize - headerSize
	for {
		msg := <-c.sendQueue

		//Split message into packets, send them all,
		i := 0

		for (len(msg.Data) - (i * maxHeadlessPacketSize)) > maxHeadlessPacketSize {
			v := (i * maxHeadlessPacketSize)
			view := msg.Data[v : v+maxHeadlessPacketSize]
			packet := c.makePacket(&msg.MsgType, &view, &packetCount, false)
			i++
			(c.tcpConn).Write(packet)
		}

		v := (i * maxHeadlessPacketSize)
		view := msg.Data[v:]
		packet := c.makePacket(&msg.MsgType, &view, &packetCount, true)
		(c.tcpConn).Write(packet)

		//collect all packets sent back.
		//Send response to caller
		EOM := false
		responses := make([][]byte, 0, 5)
		for !EOM {
			resultPacket := make([]byte, 1024, 1024)
			bytesRead, err := c.tcpConn.Read(resultPacket)
			if err != nil {
				fmt.Println(err)
				panic(err)
			}

			if resultPacket[0] != 4 {
				//Server always returns type 4 in packet header
				panic("Incorrect data")
			}
			if resultPacket[1] == 1 {
				//Byte 1 in the packet header denotes status, 1 is EOM
				//This means that there are no more responses to be collected and we can give the response back to the caller
				EOM = true
			}
			fmt.Println(bytesRead)
			fmt.Println(resultPacket[0:bytesRead])
			responses = append(responses, resultPacket[0:bytesRead])
		}

		msg.responseChannel <- &Response{Responses: &responses}
		close(msg.responseChannel)
	}
}

// Wraps the packetdata in the generic header, splits it if necessary and sends it off to the server.
func (c *SafeConn) SendMessage(msg *Message) chan (*Response) {
	result := make(chan (*Response))
	wrappedMsg := &wrappedMessage{*msg, result}
	//wrappedMsg.responseChannel = result
	go func(wrappedMsg *wrappedMessage) {
		c.sendQueue <- wrappedMsg
	}(wrappedMsg)
	return result
}

/*
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

func (c *SafeConn) makePacket(pktType *PacketType, data *[]byte, packetID *byte, lastRequest bool) []byte {
	//headerSize = 8
	result := make([]byte, headerSize, headerSize+len(*data))
	result[0] = byte(*pktType)

	if lastRequest {
		result[1] = 1
	} else {
		result[1] = 0
	}

	length := uint16(8 + len(*data))
	result[2] = byte(length / 256)
	result[3] = byte(length % 256)

	//result[4] = 0 //SP...
	//result[5] = 0 //...ID
	result[6] = *packetID
	*packetID++
	//result[7] = 0 //Window

	result = append(result, *data...)

	return result
}
