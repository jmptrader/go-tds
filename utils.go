package gotds

import (
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

/*
	Credit where credit is due, a lot of this was borrowed from the pure Go MySQL driver at https://github.com/go-sql-driver/mysql
	I'm borrowing this so I don't have to bother with DSN parsing myself right now, I will write something more in line with Microsoft's standard(s) in the future.
	The driver I mentioned is licensed under the MPL 2.0, which means we can republish this under the GPL 3.0.

	Thanks!
*/

var (
	errLog     *log.Logger    // Error Logger
	dsnPattern *regexp.Regexp // Data Source Name Parser
)

func init() {
	errLog = log.New(os.Stderr, "[go-tds] ", log.Ldate|log.Ltime|log.Lshortfile)

	dsnPattern = regexp.MustCompile(
		`^(?:(?P<user>.*?)(?::(?P<passwd>.*))?@)?` + // [user[:password]@]
			`(?:(?P<net>[^\(]*)(?:\((?P<addr>[^\)]*)\))?)?` + // [net[(addr)]]
			`\/(?P<dbname>.*?)` + // /dbname
			`(?:\?(?P<params>[^\?]*))?$`) // [?param1=value1&paramN=valueN]
}

func parseDSN(dsn string) (cfg *config, err error) {
	cfg = new(config)
	cfg.params = make(map[string]string)

	matches := dsnPattern.FindStringSubmatch(dsn)
	names := dsnPattern.SubexpNames()
	//Should validate all parameters for max length (always 128 unicode characters except for AttachDBFile)
	for i, match := range matches {
		switch names[i] {
		case "user":
			cfg.user = match
		case "passwd":
			cfg.password = match
		case "net":
			cfg.net = match
		case "addr":
			cfg.addr = match
		case "dbname":
			cfg.dbname = match
		case "params":
			for _, v := range strings.Split(match, "&") {
				param := strings.SplitN(v, "=", 2)
				if len(param) != 2 {
					continue
				}

				// cfg params
				switch value := param[1]; param[0] {
				// Dial Timeout
				case "timeout":
					cfg.timeout, err = time.ParseDuration(value)
					if err != nil {
						return
					}
				case "tls":
					boolValue, isBool := readBool(value)
					if isBool {
						cfg.verboseLog = boolValue
					}
				default:
					cfg.params[param[0]] = value
				}
			}
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

	return
}

// Returns the bool value of the input.
// The 2nd return value indicates if the input was a valid bool value
func readBool(input string) (value bool, valid bool) {
	switch input {
	case "1", "true", "TRUE", "True":
		return true, true
	case "0", "false", "FALSE", "False":
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
