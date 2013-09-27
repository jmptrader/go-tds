package gotds

import (
	"testing"
	//"net"
	//"database/sql/driver"
)

func TestOpen(t *testing.T) {
	var driver Driver
	return
	c, err := driver.Open("")
	if err != nil {
		t.Fatal(err)
		return
	}
	c.Close()
}
