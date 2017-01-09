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

func TestGenStable(t *testing.T) {
	for i := 0; i < 100; i++ {
		var out bytes.Buffer

		process(&out, bytes.NewBufferString(testinput))
		e := "13f760d2f0dc1743dd7165781f2a318d"
		if x := fmt.Sprintf("%x", md5.Sum(out.Bytes())); x != e {
			t.Errorf("got: %s wanted: %s", x, e)
		}
	}
}
