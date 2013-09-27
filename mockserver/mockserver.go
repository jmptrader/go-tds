package mockserver

import (
	"io"
	"testing"
)

type MockServer struct {
	responses       [][]byte
	t               *testing.T
	currentResponse int
}

func (m *MockServer) Read(p []byte) (n int, err error) {
	m.t.Logf("Mockread #%d", m.currentResponse)
	if m.currentResponse > len(m.responses) {
		return 0, io.EOF
	}
	response := m.responses[m.currentResponse]
	m.t.Log(response)
	destLength := len(p)
	srcLength := len(response)
	if destLength > srcLength {
		n = srcLength
	} else {
		n = destLength
	}
	copy(p, response[0:n])
	m.currentResponse++

	return
}

func (m *MockServer) Write(p []byte) (n int, err error) {
	m.t.Log(p)
	return len(p), nil
}

func (m *MockServer) Close() error {
	return nil
}

func MakeMockServer(responses [][]byte, t *testing.T) *MockServer {
	return &MockServer{responses, t, 0}
}
