package gotds

import (
	"github.com/grovespaz/go-tds/mockserver"
	"testing"
)

func TestMockPreLogin(t *testing.T) {
	mockSrv := mockserver.MakeMockServer([][]byte{[]byte{4, 1, 0, 32, 0, 0, 1, 0, 0, 0, 16, 0, 6, 1, 0, 22, 0, 1, 4, 0, 23, 0, 1, 255, 10, 50, 9, 196}}, t)

	c, err := MakeConnectionWithSocket("", mockSrv)
	if err != nil {
		t.Fatal(err)
		return
	}

	c.Close()
}

func TestLivePreLogin(t *testing.T) {
	return
	c, err := MakeConnection("")
	if err != nil {
		t.Fatal(err)
		return
	}

	c.Close()
}
