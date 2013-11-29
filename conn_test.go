package gotds

import (
	"bytes"
	_ "encoding/binary"
	"github.com/grovespaz/go-tds/mockserver"
	"reflect"
	"testing"

	utf16c "github.com/grovespaz/go-tds/utf16"
	utf16 "unicode/utf16"
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

/*
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
*/

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

func TestVariableLengthLogin(t *testing.T) {
	var clientID []byte // 6-byte, apparently created using MAC (NIC) address. No idea how though, so for now:
	clientID = []byte{0xfa, 0xca, 0xde, 0xfa, 0xca, 0xde}

	// Variable portion:
	varBlock := []varData{
		varData{strData: "host"},
		varData{strData: ensureBrackets("user")},
		varData{data: encodePassword("pass")}, //strData or data?
		varData{strData: "app"},
		varData{strData: "server"},
		varData{}, // Extension block which we do not use at the moment
		varData{strData: "driver"},
		varData{data: nil},
		varData{strData: ensureBrackets("dbname")},
		varData{data: clientID, raw: true},
		varData{}, // SSPI data, we'll look at this later...
		varData{strData: "AttachDB"},
		varData{data: []byte("newPass")},             //strData or data?
		varData{data: []byte{0, 0, 0, 0}, raw: true}, //SSPI long length.
	}

	b := makeVariableDataPortion(varBlock, 36)

	decodeVariableLengthBlock(b)
}

func TestVariableLengthLogin2(t *testing.T) {
	// From MS docs
	b := []byte{0x5E, 0x00, 0x08, 0x00, 0x6E, 0x00, 0x02, 0x00, 0x72, 0x00, 0x00, 0x00, 0x72, 0x00, 0x07, 0x00, 0x80, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x80, 0x00, 0x04, 0x00, 0x88, 0x00, 0x00, 0x00, 0x88, 0x00, 0x00, 0x00, 0x00, 0x50, 0x8B, 0xE2, 0xB7, 0x8F, 0x88, 0x00, 0x00, 0x00, 0x88, 0x00, 0x00, 0x00, 0x88, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x73, 0x00, 0x6B, 0x00, 0x6F, 0x00, 0x73, 0x00, 0x74, 0x00, 0x6F, 0x00, 0x76, 0x00, 0x31, 0x00, 0x73, 0x00, 0x61, 0x00, 0x4F, 0x00, 0x53, 0x00, 0x51, 0x00, 0x4C, 0x00, 0x2D, 0x00, 0x33, 0x00, 0x32, 0x00, 0x4F, 0x00, 0x44, 0x00, 0x42, 0x00, 0x43, 0x00}
	decodeVariableLengthBlock(b)
}

func decodeVariableLengthBlock(b []byte) {
	errLog.Printf("XX: % x \n", b)

	hostname := readBlockString(0, b)
	errLog.Printf("Hostname: %s \n", hostname)
	username := readBlockString(1*4, b)
	errLog.Printf("username: %s \n", username)
	password := decodePassword(readBlock(2*4, b))
	errLog.Printf("password: %v \n", password)
	appname := readBlockString(3*4, b)
	errLog.Printf("appname: %s \n", appname)
	servername := readBlockString(4*4, b)
	errLog.Printf("servername: %s \n", servername)
	_ = readBlock(5*4, b) //Unused
	CltIntName := readBlockString(6*4, b)
	errLog.Printf("CltIntName: %s \n", CltIntName)
	language := readBlockString(7*4, b)
	errLog.Printf("language: %s \n", language)
	database := readBlockString(8*4, b)
	errLog.Printf("database: %s \n", database)
	//sHostname := readBlockString(0, b)
}

func readBlock(off int, data []byte) []byte {
	var offset uint16
	var length uint16

	offset = (uint16(data[off+1]) * 256) + uint16(data[off+0])
	length = (uint16(data[off+3]) * 256) + uint16(data[off+2])

	//Hackish:
	offset -= 36
	x := data[offset : offset+(length)]

	return x
}

func readBlockString(off int, data []byte) string {
	// VERY inefficient, just for testing
	//buf := make([]byte, 0, 20)
	var offset uint16
	var length uint16

	offset = (uint16(data[off+1]) * 256) + uint16(data[off+0])
	length = (uint16(data[off+3]) * 256) + uint16(data[off+2])

	//Hackish:
	offset -= 36

	//errLog.Printf("off: %v, Length: %v, offset %v\n", off, length, offset)
	//x := bytes.NewReader(data[offset : offset+length])
	x := data[offset : offset+(length*2)]
	//errLog.Printf("off: %v, Length: %v, offset %v, XX: % x \n", off, length, offset, x)

	//var y []uint16
	y := make([]uint16, 0)

	errLog.Printf("%v", length)
	for i := 0; i < int(length); i++ {
		var xd uint16
		i2 := i * 2
		//err = binary.Read(x, binary.LittleEndian, xd)
		xd = (uint16(x[i2+1]) >> 8) + uint16(x[i2])
		//errLog.Printf("b: %x, c: %v", xd, string(rune(x[i2])))
		y = append(y, xd)
	}

	return string(utf16.Decode(y))
}

func decodePassword(b []byte) string {
	for i := 0; i < len(b); i++ {
		b[i] = b[i] ^ 0xA5 //10100101
		b[i] = (b[i] >> 4) | (b[i] << 4)
	}

	return utf16c.Decode(b)
}

func TestPasswordEncodeAndDecode(t *testing.T) {
	original := "test123世界blaat"
	encoded := encodePassword(original)
	decoded := decodePassword(encoded)
	if decoded != original {
		t.Fatalf("Original and decoded doesn't match, %v (% x) vs. %v (% x)", original, original, decoded, decoded)
	}
}
