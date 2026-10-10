package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		e := "8372a045a3d73af272913fdcbb82f62b"
		if x := fmt.Sprintf("%x", md5.Sum(out.Bytes())); x != e {
			t.Errorf("got: %s wanted: %s", x, e)
		}
	}
}

func runGeneratedLexerProgram(t *testing.T, nexSpec string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "nex_terminal_nl_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	var out bytes.Buffer
	process(&out, strings.NewReader(nexSpec))

	mainPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainPath, out.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write generated code: %v", err)
	}

	cmd := exec.Command("go", "run", "main.go")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("test execution failed: %v\nOutput: %s", err, string(output))
	}
	if strings.TrimSpace(string(output)) != "PASS" {
		t.Fatalf("unexpected output: %s", string(output))
	}
}

// TestTerminalNewlineWithoutLookahead verifies that a single line input
// terminated by '\n' on an open pipe emits tokens immediately without hanging on lookahead.
func TestTerminalNewlineWithoutLookahead(t *testing.T) {
	spec := `
/[0-9]+/ { return 1 }
/[ \t]+/ { /* skip */ }
/\+/     { return 2 }
/\n/     { return 10 }
/./      { return int(yylex.Text()[0]) }
//
package main

import (
	"fmt"
	"os"
	"time"
)

type yySymType struct {
	val int
}

func main() {
	r, w := io.Pipe()

	go func() {
		// Single line terminated by newline; pipe is left open intentionally.
		w.Write([]byte("42 + 58\n"))
	}()

	lex := NewLexer(r)
	tokenCh := make(chan int, 10)
	go func() {
		for {
			var lval yySymType
			tok := lex.Lex(&lval)
			tokenCh <- tok
			if tok == 10 || tok == 0 {
				break
			}
		}
	}()

	expected := []int{1, 2, 1, 10}
	for _, exp := range expected {
		select {
		case tok := <-tokenCh:
			if tok != exp {
				fmt.Printf("FAIL: got token %d, want %d\n", tok, exp)
				os.Exit(1)
			}
		case <-time.After(3 * time.Second):
			fmt.Printf("FAIL: timeout waiting for token %d (hung on lookahead)\n", exp)
			os.Exit(2)
		}
	}
	fmt.Println("PASS")
}
`
	runGeneratedLexerProgram(t, spec)
}

// TestIssue56RPProgram verifies Dan Swartzendruber's exact test case (issue #56):
// in rp.nex, single line "111 111 * 3 /\n" emits all tokens including terminal '\n'
// matched by /./ without requiring a second newline to unblock lookahead.
func TestIssue56RPProgram(t *testing.T) {
	spec := `
/[ \t]/  { /* skip */ }
/[0-9]+/ { return 1 }
/./      { return int(yylex.Text()[0]) }
//
package main

import (
	"fmt"
	"os"
	"time"
)

type yySymType struct {
	val int
}

func main() {
	r, w := io.Pipe()

	go func() {
		w.Write([]byte("111 111 * 3 /\n"))
	}()

	lex := NewLexer(r)
	tokenCh := make(chan int, 10)
	go func() {
		for {
			var lval yySymType
			tok := lex.Lex(&lval)
			tokenCh <- tok
			if tok == int('\n') || tok == 0 {
				break
			}
		}
	}()

	expected := []int{1, 1, int('*'), 1, int('/'), int('\n')}
	for _, exp := range expected {
		select {
		case tok := <-tokenCh:
			if tok != exp {
				fmt.Printf("FAIL: got token %d, want %d\n", tok, exp)
				os.Exit(1)
			}
		case <-time.After(3 * time.Second):
			fmt.Printf("FAIL: timeout waiting for token %d (hung on lookahead)\n", exp)
			os.Exit(2)
		}
	}
	fmt.Println("PASS")
}
`
	runGeneratedLexerProgram(t, spec)
}

// TestTerminalNewlineEmptyLine verifies that an empty line with just '\n'
// immediately emits the newline token on an unclosed pipe.
func TestTerminalNewlineEmptyLine(t *testing.T) {
	spec := `
/\n/ { return 10 }
/./  { return int(yylex.Text()[0]) }
//
package main

import (
	"fmt"
	"os"
	"time"
)

type yySymType struct{}

func main() {
	r, w := io.Pipe()

	go func() {
		w.Write([]byte("\n"))
	}()

	lex := NewLexer(r)
	tokenCh := make(chan int, 2)
	go func() {
		var lval yySymType
		tok := lex.Lex(&lval)
		tokenCh <- tok
	}()

	select {
	case tok := <-tokenCh:
		if tok != 10 {
			fmt.Printf("FAIL: got token %d, want 10\n", tok)
			os.Exit(1)
		}
	case <-time.After(3 * time.Second):
		fmt.Println("FAIL: timeout waiting for newline token")
		os.Exit(2)
	}
	fmt.Println("PASS")
}
`
	runGeneratedLexerProgram(t, spec)
}

// TestSequentialLinesStreamed verifies multiple lines streamed sequentially
// over an unclosed pipe emit each line's tokens promptly without buffering delays.
func TestSequentialLinesStreamed(t *testing.T) {
	spec := `
/[a-z]+/ { return 1 }
/\n/     { return 10 }
/./      { return int(yylex.Text()[0]) }
//
package main

import (
	"fmt"
	"os"
	"time"
)

type yySymType struct{}

func main() {
	r, w := io.Pipe()

	lex := NewLexer(r)
	tokenCh := make(chan int, 10)
	go func() {
		for {
			var lval yySymType
			tok := lex.Lex(&lval)
			tokenCh <- tok
			if tok == 0 {
				break
			}
		}
	}()

	// First line
	w.Write([]byte("foo\n"))
	for _, exp := range []int{1, 10} {
		select {
		case tok := <-tokenCh:
			if tok != exp {
				fmt.Printf("FAIL: line 1 got %d, want %d\n", tok, exp)
				os.Exit(1)
			}
		case <-time.After(3 * time.Second):
			fmt.Println("FAIL: line 1 timeout waiting for token")
			os.Exit(2)
		}
	}

	// Second line
	w.Write([]byte("bar\n"))
	for _, exp := range []int{1, 10} {
		select {
		case tok := <-tokenCh:
			if tok != exp {
				fmt.Printf("FAIL: line 2 got %d, want %d\n", tok, exp)
				os.Exit(1)
			}
		case <-time.After(3 * time.Second):
			fmt.Println("FAIL: line 2 timeout waiting for token")
			os.Exit(2)
		}
	}

	fmt.Println("PASS")
}
`
	runGeneratedLexerProgram(t, spec)
}
