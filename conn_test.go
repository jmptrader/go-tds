package gotds

import (
	"bytes"
	"github.com/grovespaz/go-tds/mockserver"
	"reflect"
	"testing"
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
	config, _ := parseDSN("gotest:gotest@(slu.is:49286)/gotest")

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
	testTokens[0] = token{definition: tokenDefinition(zeroLength)}
	testTokens[1] = token{definition: tokenDefinition(fixedLength), data: []byte{0xff}}
	testTokens[2] = token{definition: tokenDefinition(fixedLength), data: []byte{0xfe, 0xfd}}
	testTokens[3] = token{definition: tokenDefinition(fixedLength), data: []byte{0xfc, 0xfb, 0xfa, 0xf9}}
	testTokens[4] = token{definition: tokenDefinition(fixedLength), data: []byte{0xf8, 0xf7, 0xf6, 0xf5, 0xf4, 0xf3, 0xf2, 0xf1}}

	rawTokens, err := makeTokenStream(testTokens)
	errLog.Printf("RawTokens: %v\n", rawTokens)
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

	if (simple[0].definition != tokenDefinition(zeroLength)) || (simple[0].data != nil) {
		t.Fatal("Parsing incorrect for simple token 0")
	}

	if (simple[1].definition != tokenDefinition(fixedLength)) || (len(simple[1].data) != 1) || (simple[1].data[0] != 0xff) {
		t.Fatal("Parsing incorrect for simple token 1")
	}

	if (simple[2].definition != tokenDefinition(fixedLength|0x4)) || (len(simple[2].data) != 2) || !(bytes.Equal(simple[2].data, []byte{0xfe, 0xfd})) {
		t.Log(simple[2].data)
		t.Log([]byte{0xfe, 0xfd})
		t.Fatal("Parsing incorrect for simple token 2")
	}

	if (simple[3].definition != tokenDefinition(fixedLength|0x8)) || (len(simple[3].data) != 4) || !(bytes.Equal(simple[3].data, []byte{0xfc, 0xfb, 0xfa, 0xf9})) {
		t.Fatal("Parsing incorrect for simple token 3")
	}

	if (simple[4].definition != tokenDefinition(fixedLength|0xc)) || (len(simple[4].data) != 8) || !(bytes.Equal(simple[4].data, []byte{0xf8, 0xf7, 0xf6, 0xf5, 0xf4, 0xf3, 0xf2, 0xf1})) {
		t.Fatal("Parsing incorrect for simple token 4")
	}
}

func TestVariableLengthTokenBuildingAndParsing(t *testing.T) {
	testTokens := make([]token, 5)
	//Note: Specifying the length here is unnecessary since it will be overwritten in makeTokenStream(). It is, however, needed so we can cheat and use reflect.DeepEqual to verify the result.
	testTokens[0] = token{definition: tokenDefinition(fixedLength), length: 1, data: []byte{0xff}}
	testTokens[1] = token{definition: tokenDefinition(variableLength), length: 24, data: []byte("Teststring, hello world!")}
	testTokens[2] = token{definition: tokenDefinition(fixedLength | 0x4), length: 2, data: []byte{0xfe, 0xfd}}
	testTokens[3] = token{definition: tokenDefinition(variableLength), length: 18, data: []byte("Another teststring")}
	testTokens[4] = token{definition: tokenDefinition(variableLength), length: 8, data: []byte{0xf8, 0xf7, 0xf6, 0xf5, 0xf4, 0xf3, 0xf2, 0xf1}}

	rawTokens, err := makeTokenStream(testTokens)
	if err != nil {
		t.Fatal(err)
		return
	}

	errLog.Printf("RawTokens: %v\n", rawTokens)
	parsedTokens, err := parseTokenStream(rawTokens)

	if !reflect.DeepEqual(testTokens, parsedTokens) {
		t.Log(testTokens)
		t.Log(parsedTokens)
		t.Fatal("Variable length building or parsing failed")
	}
}
