package gotds

import (
	"github.com/grovespaz/go-tds/mockserver"
	"testing"
	"bytes"
)

func TestMockPreLogin(t *testing.T) {
	mockSrv := mockserver.MakeMockServer([][]byte{[]byte{4, 1, 0, 32, 0, 0, 1, 0, 0, 0, 16, 0, 6, 1, 0, 22, 0, 1, 4, 0, 23, 0, 1, 255, 10, 50, 9, 196}}, t)
	config, _ := parseDSN("/")

	c, err := MakeConnectionWithSocket(config, mockSrv)
	if err != nil {
		t.Fatal(err)
		return
	}

	c.Close()
}

func TestLivePreLogin(t *testing.T) {
	return
	config, _ := parseDSN("otest:gotest@(slu.is:49286)/gotest")

	c, err := MakeConnection(config)
	if err != nil {
		t.Fatal(err)
		return
	}

	c.Close()
}

func TestTokenParsing(t *testing.T) {
	// Blank stream:
	testData := make([]byte, 0)
	blank, err := parseTokenStream(testData)
	if err != nil {
		t.Fatal(err)
		return
	}

	if len(blank) != 0 {
		t.Fatal("Length incorrect for blank token")
	}

	// Simple stream, testing zero-length and fixed-length tokens:
	testData = []byte{byte(zeroLength)}
	testData = append(testData, []byte{byte(fixedLength), 0xff}...)
	testData = append(testData, []byte{byte(fixedLength) | 0x4, 0xfe, 0xfd}...)
	testData = append(testData, []byte{byte(fixedLength) | 0x8, 0xfc, 0xfb, 0xfa, 0xf9}...)
	testData = append(testData, []byte{byte(fixedLength) | 0xc, 0xf8, 0xf7, 0xf6, 0xf5, 0xf4, 0xf3, 0xf2, 0xf1}...)
	simple, err := parseTokenStream(testData)
	VerifyCorrectSimpleTokens(simple, err, t)
}

func TestTokenBuildingAndParsing(t *testing.T) {
	testTokens := make([]token, 5)
	testTokens[0] = token{definition: zeroLength}
	testTokens[1] = token{definition: fixedLength, data: []byte{0xff}}
	testTokens[2] = token{definition: fixedLength, data: []byte{0xfe, 0xfd}}
	testTokens[3] = token{definition: fixedLength, data: []byte{0xfc, 0cfb, 0xfa, 0xf9}}
	testTokens[4] = token{definition: fixedLength, data: []byte{0xf8, 0xf7, 0xf6, 0xf5, 0xf4, 0xf3, 0xf2, 0xf1}}
	
	rawTokens, err := makeTokenStream(testTokens)
	
	simple, err := parseTokenStream(rawTokens)
	VerifyCorrectSimpleTokens(simple, err, t)
}

func VerifyCorrectSimpleTokens(simple []token, err error, t *testing.T) {
	if err != nil {
		t.Fatal(err)
		return
	}

	if len(simple) != 5 {
		t.Fatal("Length incorrect for simple token")
	}

	if (simple[0].definition != zeroLength) || (simple[0].data != nil) {
		t.Fatal("Parsing incorrect for simple token 0")
	}
	
	if (simple[1].definition != fixedLength) || (len(simple[1].data) != 1) || (simple[1].data[0] != 0xff) {
		t.Fatal("Parsing incorrect for simple token 1")
	}
	
	if (simple[2].definition != fixedLength) || (len(simple[2].data) != 2) || !(bytes.Equal(simple[2].data, []byte{0xfe, 0xfd})) {
		t.Log(simple[2].data)
		t.Log([]byte{0xfe, 0xfd})
		t.Fatal("Parsing incorrect for simple token 2")
	}
	
	if (simple[3].definition != fixedLength) || (len(simple[3].data) != 4) || !(bytes.Equal(simple[3].data, []byte{0xfc, 0xfb, 0xfa, 0xf9})) {
		t.Fatal("Parsing incorrect for simple token 3")
	}
	
	if (simple[4].definition != fixedLength) || (len(simple[4].data) != 8) || !(bytes.Equal(simple[4].data, []byte{0xf8, 0xf7, 0xf6, 0xf5, 0xf4, 0xf3, 0xf2, 0xf1})) {
		t.Fatal("Parsing incorrect for simple token 4")
	}
}