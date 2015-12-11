package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"testing"
)

var testinput = `
/a|A/ { return A }
//
package main
`

func TestGen(t *testing.T) {
	var out bytes.Buffer

	process(&out, bytes.NewBufferString(testinput))
	e := "f812a91c49951a549c3faa3ba715b45e"
	if x := fmt.Sprintf("%x", md5.Sum(out.Bytes())); x != e {
		t.Errorf("got: %s wanted: %s", x, e)
	}

}
