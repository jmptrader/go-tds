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
