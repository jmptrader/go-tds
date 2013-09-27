package safeconn

import (
	"testing"
	"github.com/grovespaz/go-tds/mockserver"
)

func TestPreLoginPacket(t *testing.T) {
	mockSrv := mockserver.MakeMockServer(nil, t)

	safeConn := &SafeConn{tcpConn: mockSrv, sendQueue: make(chan *wrappedMessage), maxPacketSize: 18}
	go safeConn.sendLoop()
	
	msg1 := &Message{1, make([]byte, 10)}
	chan1 := safeConn.SendMessage(msg1)
	
	msg2 := &Message{2, make([]byte, 20)}
	for i := 0; i < 20; i++ {
		msg2.data[i] = byte(i + 1)
	}
	chan2 := safeConn.SendMessage(msg2)

	msg3 := &Message{3, make([]byte, 15)}
	for i := 0; i < 15; i++ {
		msg3.data[i] = byte(i + 1)
	}
	chan3 := safeConn.SendMessage(msg3)
	
	<-chan1
	<-chan2
	<-chan3
	//safeConn.Close()
}