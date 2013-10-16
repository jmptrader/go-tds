package gotds

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os" // For hostname
)

func (c *Conn) login() ([]byte, error) {
	loginPacket, err := c.makeLoginPacket()
	if err != nil {
		return nil, err
	}

	if c.cfg.verboseLog {
		errLog.Printf("Trying to login with username: %v, password: %v and default DB: %v", c.cfg.user, c.cfg.password, c.cfg.dbname)
		errLog.Printf("Request: %X\n", loginPacket)
	}

	loginResult, err := c.SendMessage(ptyLogin, loginPacket)

	if err != nil {
		return nil, err
	}

	if len(*loginResult) == 0 {
		return nil, errors.New("No preLogin response")
	}

	if len(*loginResult) > 1 {
		return nil, errors.New("More than 1 result in the preLogin response")
	}

	loginResultData := (*loginResult)[0] //[8:]

	if c.cfg.verboseLog {
		errLog.Printf("Request: %v\n", loginPacket)
		errLog.Printf("Response: %v\n", loginResultData)
	}

	return loginResultData, nil
}

func (c *Conn) makeLoginPacket() ([]byte, error) {
	b := new(bytes.Buffer)
	b.Grow(0) // TODO: Fill in the least needed amount here

	b.Write([]byte{0, 0, 0, 0}) // Length bytes, to be set later

	binary.Write(b, binary.BigEndian, c.tdsVersion)
	binary.Write(b, binary.LittleEndian, c.maxPacketSize)
	binary.Write(b, binary.LittleEndian, driverVersion)
	binary.Write(b, binary.LittleEndian, c.clientPID)
	binary.Write(b, binary.LittleEndian, c.connectionID)

	optionFlags1 := makeByteFromBits(c.byteOrder,
		c.charType,
		false, //Floattype: 2 bits, 00 = IEEE_754
		false,
		c.dumpLoad,
		c.useDBWarnings,
		c.cfg.failIfNoDB,
		c.setLang)
	b.WriteByte(optionFlags1)

	// I cheat here a little bit because we know the user is always going to be a regular user. Hence, the hardcoded false-flags you see here are specifying the usertype regular user.
	optionFlags2 := makeByteFromBits(c.cfg.failIfNoLanguage,
		c.odbc,
		false, //transboundary?
		false, //cacheConnect?
		false,
		false,
		false,
		c.cfg.integratedSecurity)
	b.WriteByte(optionFlags2)

	if (c.tdsVersion < TDS72) && (c.useOLEDB) {
		panic("Cannot set useOLEDB when TDSVersion < 7.2")
	}

	// Readonly but can still be sent < TDS7.4 even if it was only introduced in 7.4

	typeFlags := makeByteFromBits(c.cfg.sqlType,
		false, //SQLType is documented to be 4-bits, only 1 is used.
		false,
		false,
		c.useOLEDB,
		c.cfg.readOnly,
		false,
		false)

	b.WriteByte(typeFlags)

	if c.tdsVersion < TDS72 {
		b.WriteByte(0) // Was reserved < TDS7.2
	} else {
		optionFlags3 := makeByteFromBits(c.cfg.changePass,
			false, //for now, Determines if Yukon binary xml is sent when sending XML
			c.cfg.userInstance,
			false, //unknown collation handling pre 7.3
			false, //Do we use the extension-section introduced in 7.4? No we don't cause we don't offer connection resuming.
			false, //Unused from here
			false,
			false)
		b.WriteByte(optionFlags3)
	}

	binary.Write(b, binary.LittleEndian, c.cfg.timezone)
	binary.Write(b, binary.LittleEndian, c.cfg.lcid)

	hostname, err := os.Hostname()
	if err != nil {
		// Not strictly necessary, we can send a nil value but meh.
		hostname = "Unknown-go-tds-client"
	}

	var appname string
	if c.cfg.appname != "" {
		appname = c.cfg.appname
	} else {
		appname = os.Args[0] // Should be executable name, at least in *nix
	}

	var servername string
	var clientID []byte // 6-byte, apparently created using MAC (NIC) address. No idea how though, so for now:
	clientID = []byte{0xfa, 0xca, 0xde, 0xfa, 0xca, 0xde}

	// Variable portion:
	varBlock := []varData{
		varData{data: []byte(hostname)},
		varData{data: []byte(ensureBrackets(c.cfg.user))},
		varData{data: encodePassword(c.cfg.password)},
		varData{data: []byte(appname)},
		varData{data: []byte(servername)},
		varData{}, // Extension block which we do not use at the moment
		varData{data: []byte(driverName)},
		varData{data: []byte(c.cfg.preferredLanguage)},
		varData{data: []byte(ensureBrackets(c.cfg.dbname))},
		varData{data: clientID, raw: true},
		varData{}, // SSPI data, we'll look at this later...
		varData{data: []byte(c.cfg.attachDB)},
		varData{data: []byte(c.cfg.newPass)},
		varData{data: []byte{0, 0, 0, 0}, raw: true}, //SSPI long length.
	}

	b.Write(makeVariableDataPortion(varBlock, b.Len()))

	// Have to write length as first 4 bytes:
	// Even though we'll never exceed 2 bytes...
	result := b.Bytes()
	length := len(result)
	result[0] = byte(length % 256)
	result[1] = byte(length / 256)

	return result, nil
}

// The second part of the LOGIN message contains all data of variable length (mostly strings)
// The result consists of two parts, a header indicating all offsets and lengths, and the actual data following that.
// For some reason I can't fathom, smack in the middle of the header lies a 6(!)-byte field for the ClientID, which completely breaks any sleek generic function one would want to write for this. At the end of the header is another field in case the SSPI-length was larger than uint16. This field is a uint32 and can be used as a replacement length.
// Because of this, I introduce this struct:
type varData struct {
	data []byte // The data to include
	raw  bool   // Whether to do it properly or to just smack the raw data in the header...
}

//...which we loop through a couple of times here
func makeVariableDataPortion(data []varData, startingOffset int) []byte {
	totalLength := 0
	for _, part := range data {
		dataLength := len(part.data)
		if part.raw {
			startingOffset += dataLength
			totalLength += 4 + dataLength
		} else {
			startingOffset += 4 //Two bytes offset, two bytes length
			totalLength += 4 + dataLength
		}
	}

	offset := startingOffset
	buf := bytes.NewBuffer(make([]byte, 0, totalLength))

	for _, part := range data {
		if part.raw {
			buf.Write(part.data)
		} else {
			dataLength := len(part.data)
			//if dataLength != 0 {
			binary.Write(buf, binary.LittleEndian, uint16(offset))
			binary.Write(buf, binary.LittleEndian, uint16(dataLength))
			//} else {
			//	buf.Write([]byte{0, 0, 0, 0})
			//}
			//Todo: Find out if (when sending a blank entry) the offset needs to be 0 too, I believe so but the official MS example does differently.
			offset += dataLength
		}
	}

	for _, part := range data {
		if !part.raw {
			buf.Write(part.data)
		}
	}

	return buf.Bytes()
}

func encodePassword(password string) []byte {
	b := []byte(password)
	for i := 0; i < len(b); i++ {
		b[i] = (b[i] >> 4) & (b[i] << 4)
		b[i] = b[i] ^ 0xA5 //10100101
	}

	return b
}

// ensureBrackets ensures that a value is enclosed in square brackets like so: [value]
// Later on this could be changed into a more full-fledged validator for object identifiers (for values such as [dbo].[value]
func ensureBrackets(value string) string {
	if (value[0] == '[') && (value[len(value)-1] == ']') {
		return value
	}

	if (value[0] != '[') && (value[len(value)-1] != ']') {
		return "[" + value + "]"
	}

	panic("Incorrect format specified for value: " + value)
}
