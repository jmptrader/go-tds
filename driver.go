package gotds

import (
	//"net"
	"database/sql/driver"
	//"time"
)

type Driver struct {
}

func (driver *Driver) Open(name string) (driver.Conn, error) {
	return MakeConnection(name)
	//return nil, driver.ErrBadConn
}