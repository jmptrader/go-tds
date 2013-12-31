package gotds

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

/*
Code in this file was borrowed and modified from https://github.com/ziutek/mymysql/
which includes the following LICENSE:

Copyright (c) 2010, Michal Derkacz
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions
are met:
1. Redistributions of source code must retain the above copyright
   notice, this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright
   notice, this list of conditions and the following disclaimer in the
   documentation and/or other materials provided with the distribution.
3. The name of the author may not be used to endorse or promote products
   derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE AUTHOR ''AS IS'' AND ANY EXPRESS OR
IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES
OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT, INDIRECT,
INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT
NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF
THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

func escapeParameters(query string, args []driver.Value, placeholder rune) (string, error) {
	//TODO(gv): Optimize this
	if len(args) == 0 {
		return query, nil
	}
	q := make([]string, 2*len(args)+1)
	n := 0
	for _, a := range args {
		i := strings.IndexRune(query, placeholder)
		if i == -1 {
			return "", errors.New("number of parameters doesn't match number of placeholders")
		}
		var s string
		switch v := a.(type) {
		case nil:
			s = "NULL"
		case string:
			s = "'" + escapeString(v) + "'"
		case []byte:
			// TODO(gv): Find out if this works:
			s = "'" + escapeString(string(v)) + "'"
		case int64:
			s = strconv.FormatInt(v, 10)
		case time.Time:
			//s = "'" + v.Format(mysql.TimeFormat) + "'"
		case bool:
			if v {
				s = "1"
			} else {
				s = "0"
			}
		case float64:
			s = strconv.FormatFloat(v, 'e', 12, 64)
		default:
			panic(fmt.Sprintf("%v (%T) can't be handled by godrv"))
		}
		q[n] = query[:i]
		q[n+1] = s
		query = query[i+1:]
		n += 2
	}
	q[n] = query
	return strings.Join(q, ""), nil
}

func escapeString(txt string) string {
	// Thanks https://github.com/ziutek/mymysql/
	// Replaced filters with my own, t-sql seems simpler (allow everything except ').
	// TODO(gv): Verify that these are the only characters which need to be escaped and that nul actually needs to be escaped
	// Otherwise just strReplace ' with ''.
	var (
		esc string
		buf bytes.Buffer
	)
	last := 0
	for ii, bb := range txt {
		switch bb {
		case 0:
			esc = `\0`
		case '\'':
			esc = `''`
		default:
			continue
		}
		io.WriteString(&buf, txt[last:ii])
		io.WriteString(&buf, esc)
		last = ii + 1
	}
	io.WriteString(&buf, txt[last:])
	return buf.String()
}
