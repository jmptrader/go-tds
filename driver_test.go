package gotds

import (
	"testing"
	//"net"
	"database/sql"
	//"database/sql/driver"
)

func TestInternalDriverOpen(t *testing.T) {
	var driver Driver
	return
	c, err := driver.Open("")
	if err != nil {
		t.Fatal(err)
		return
	}
	c.Close()
}

func TestDriverOpen(t *testing.T) {
	db, err := sql.Open("tds", "Data Source=slu.is:49286;Initial Catalog=gotest;uid=gotest;pwd=gotest")
	if err != nil {
		t.Fatal(err)
		return
	}

	// Open doesn't (always) open a connection. This does:
	err = db.Ping()
	if err != nil {
		t.Fatal(err)
		return
	}
}
