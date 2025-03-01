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
		e := "f2fef8e2a1c5eeb7c38f6e437be9ccac"
		if x := fmt.Sprintf("%x", md5.Sum(out.Bytes())); x != e {
			t.Errorf("got: %s wanted: %s", x, e)
		}
	}
}
