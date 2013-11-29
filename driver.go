// go-tds is an experimental pure-go TDS-driver.
// With it you should be able to connect to Microsoft SQL Servers (2000, 2005, 2008, 2012 and up).
// It is not functional yet.
package gotds

import (
	//"net"
	"database/sql"
	"database/sql/driver"
	//"time"
)

type Driver struct {
}

func (driver *Driver) Open(dsn string) (driver.Conn, error) {
	config, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}

	return MakeConnection(config)
	//return nil, driver.ErrBadConn
}

func init() {
	sql.Register("tds", &Driver{})
}
