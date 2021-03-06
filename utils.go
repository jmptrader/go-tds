package gotds

import (
	"errors"
	"io"
	"log"
	"os"
	//"regexp"
	"bytes"
	"encoding/binary"
	utf16c "github.com/Grovespaz/go-tds/utf16"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

var (
	errLog *log.Logger // Error Logger
	//dsnPattern *regexp.Regexp // Data Source Name Parser
)

func init() {
	logFile, err := os.Create("debug.log")
	if err != nil {
		panic(err)
	}

	multiLog := io.MultiWriter(os.Stderr, logFile)

	errLog = log.New(multiLog, "[go-tds] ", log.Ldate|log.Ltime|log.Lshortfile)
}

func parseDSN(dsn string) (cfg *config, err error) {
	cfg = new(config)
	cfg.params = make(map[string]string)
	//cfg.verboseLog = true

	dsn = strings.ToLower(dsn)

	for _, v := range strings.Split(dsn, ";") {
		s := strings.SplitN(v, "=", 2)
		value := s[1]
		//Should validate all parameters for max length (always 128 unicode characters except for AttachDBFile)

		switch s[0] {
		case "user id":
			fallthrough
		case "uid":
			fallthrough
		case "username":
			fallthrough
		case "user":
			cfg.user = value

		case "password":
			fallthrough
		case "pwd":
			cfg.password = value

		case "net":
			cfg.net = value

		case "data source":
			fallthrough
		case "server":
			fallthrough
		case "address":
			fallthrough
		case "network address":
			fallthrough
		case "addr":
			cfg.addr = value
		case "initial catalog":
			fallthrough
		case "database":
			cfg.dbname = value

		case "connect timeout":
			fallthrough
		case "connection timeout":
			cfg.timeout, err = time.ParseDuration(value)
			if err != nil {
				return
			}
		case "verbose":
			boolValue, isBool := readBool(value)
			if isBool {
				cfg.verboseLog = boolValue
			}
		case "application name":
			cfg.appname = value

		case "extended properties":
			fallthrough
		case "initial file name":
			fallthrough
		case "attachdbfilename":
			cfg.appname = value

		case "packet size":
			var i int
			i, err = strconv.Atoi(value)
			if err != nil {
				return
			}
			cfg.maxPacketSize = uint32(i)
		case "encrypt":
			// This is a funny one. We can only turn encryption on or off here, apparently.
			// But even if we turn it off, official MS drivers will still exchange certificate information to transfer the login information (thus is the behaviour of encryptOff apparently).
			// The server also expects this.
			// Only when we specify encryption to be completely unsupported (encryptNotSupported) will we completely skip encryption.
			// However, this is not officially supported as a connection string option by MS (because, why would you?).
			// Thus I add it as a custom option (not_supported) should anyone want it.
			// Also, I haven't looked at what 'true' does in this context, encryptOn or encryptRequired.
			// For good measure true = encryptOn and 'required' = encryptRequired

			// Note that this option is currently ignored as I haven't looked at TLS/SSL connections.
			boolValue, isBool := readBool(value)
			if isBool {
				if boolValue {
					cfg.encryption = encryptOn // Is this right?
				} else {
					cfg.encryption = encryptOff
				}
			} else {
				switch value {
				case "not_supported":
					cfg.encryption = encryptNotSupported
				case "required":
					cfg.encryption = encryptRequired
				default:
					cfg.encryption = encryptOn
				}
			}
		case "trustservercertificate":
			boolValue, isBool := readBool(value)
			if isBool {
				cfg.trustServerCertificate = boolValue
			}
		case "placeholder":
			if len(value) != 1 {
				return nil, errors.New("Invalid placeholder char")
			}
			cfg.placeholder = []rune(value)[0]
		default:
			cfg.params[s[0]] = value
		}
	}

	// Set default network if empty
	if cfg.net == "" {
		cfg.net = "tcp"
	}

	// Set default adress if empty
	if cfg.addr == "" {
		cfg.addr = "127.0.0.1:1433"
	}

	if cfg.maxPacketSize == 0 {
		cfg.maxPacketSize = 0x1000
	}

	if cfg.placeholder == 0 {
		cfg.placeholder = '?'
	}

	return
}

// Returns the bool value of the input.
// The 2nd return value indicates if the input was a valid bool value
func readBool(input string) (value bool, valid bool) {
	switch strings.ToLower(input) {
	case "1", "true", "yes":
		return true, true
	case "0", "false", "no":
		return false, true
	}

	// Not a valid bool value
	return
}

func makeByteFromBits(b1 bool, b2 bool, b3 bool, b4 bool, b5 bool, b6 bool, b7 bool, b8 bool) (result byte) {
	if b1 {
		result = 1
	}
	if b2 {
		result |= 2
	}
	if b3 {
		result |= 4
	}
	if b4 {
		result |= 8
	}
	if b5 {
		result |= 16
	}
	if b6 {
		result |= 32
	}
	if b7 {
		result |= 64
	}
	if b8 {
		result |= 128
	}
	return
}

func writeUTF16String(w io.Writer, s string) error {
	utfString := utf16.Encode([]rune(s))
	return binary.Write(w, binary.LittleEndian, utfString)
}

func readUS_VarChar(buf *bytes.Buffer) string {
	// Should be null-aware here
	var txtLength uint16
	err := binary.Read(buf, binary.LittleEndian, &txtLength)
	if err != nil {
		panic(err)
	}

	if txtLength > 0 {
		rawMsg := buf.Next(int(txtLength) * 2)
		return utf16c.Decode(rawMsg)
	} else {
		return ""
	}
}

func readB_VarChar(buf *bytes.Buffer) string {
	txtLength, err := buf.ReadByte()

	if err != nil {
		panic(err)
	}

	if txtLength > 0 {
		rawMsg := buf.Next(int(txtLength) * 2)
		return utf16c.Decode(rawMsg)
	} else {
		return ""
	}
}
