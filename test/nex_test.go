package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var nexBin string

func init() {
  var err error
  if nexBin, err = filepath.Abs(os.Getenv("GOPATH") + "/bin/nex"); err != nil {
    panic(err)
  }
  if _, err := os.Stat(nexBin); err != nil {
    if nexBin, err = filepath.Abs("../nex"); err != nil {
      panic(err)
    }
    if _, err := os.Stat(nexBin); err != nil {
      panic("cannot find nex binary")
    }
  }
}

func dieErr(t *testing.T, err error, s string) {
	if err != nil {
		t.Fatalf("%s: %s", s, err)
	}
}

// Test the reverse-Polish notation calculator rp.{nex,y}.
func TestNexPlusYacc(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "nex")
	dieErr(t, err, "TempDir")
	defer func() {
		dieErr(t, os.RemoveAll(tmpdir), "RemoveAll")
	}()
	run := func(s string) {
		v := strings.Split(s, " ")
		err := exec.Command(v[0], v[1:]...).Run()
		dieErr(t, err, s)
	}
	run("cp rp.nex rp.y " + tmpdir)
	wd, err := os.Getwd()
	dieErr(t, err, "Getwd")
	dieErr(t, os.Chdir(tmpdir), "Chdir")
	defer func() {
		dieErr(t, os.Chdir(wd), "Chdir")
	}()
	run(nexBin + " rp.nex")
	run("go tool yacc rp.y")
	run("go build y.go rp.nn.go")
	cmd := exec.Command("./y")
	cmd.Stdin = strings.NewReader(
		`1 2 3 4 + * -
9 8 * 7 * 3 2 * 1 * / n
`)
	want := "-13\n-84\n"
	got, err := cmd.CombinedOutput()
	dieErr(t, err, "CombinedOutput")
	if want != string(got) {
		t.Fatalf("want %q, got %q", want, string(got))
	}
}

func TestNexPrograms(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "nex")
	dieErr(t, err, "TempDir")
	defer func() {
		dieErr(t, os.RemoveAll(tmpdir), "RemoveAll")
	}()

	for _, x := range []struct {
		prog, in, out string
	}{
		{"lc.nex", "no newline", "0 10\n"},
		{"lc.nex", "one two three\nfour five six\n", "2 28\n"},

		{"toy.nex", "if\t\n6 * 9 ==   42  then {one-line comment } else 1.23. end",
			`A keyword: if
An integer: 6
An operator: *
An integer: 9
Unrecognized character: =
Unrecognized character: =
An integer: 42
A keyword: then
An identifier: else
A float: 1.23
Unrecognized character: .
A keyword: end
`},

		{"wc.nex", "no newline", "0 0 0\n"},
		{"wc.nex", "\n", "1 0 1\n"},
		{"wc.nex", "1\na b\nA B C\n", "3 6 12\n"},
		{"wc.nex", "one two three\nfour five six\n", "2 6 28\n"},

		{"rob.nex",
			`1 robot
2 robo
3 rob
4 rob robot
5 robot rob
6 roboot
`, "2 robo\n3 rob\n6 roboot\n"},

		{"peter.nex",
			`    #######
   #########
  ####  #####
 ####    ####   #
 ####      #####
####        ###
########   #####
#### #########
#### #  # ####
## #  ###   ##
###    #  ###
###    ##
 ##   #
  #   ####
  # #
##   #   ##
`,
			`rect 5 6 1 2
rect 6 7 1 2
rect 7 8 1 2
rect 8 9 1 2
rect 9 10 1 2
rect 10 11 1 2
rect 11 12 1 2
rect 4 5 2 3
rect 5 6 2 3
rect 6 7 2 3
rect 7 8 2 3
rect 8 9 2 3
rect 9 10 2 3
rect 10 11 2 3
rect 11 12 2 3
rect 12 13 2 3
rect 3 4 3 4
rect 4 5 3 4
rect 5 6 3 4
rect 6 7 3 4
rect 9 10 3 4
rect 10 11 3 4
rect 11 12 3 4
rect 12 13 3 4
rect 13 14 3 4
rect 2 3 4 5
rect 3 4 4 5
rect 4 5 4 5
rect 5 6 4 5
rect 10 11 4 5
rect 11 12 4 5
rect 12 13 4 5
rect 13 14 4 5
rect 17 18 4 5
rect 2 3 5 6
rect 3 4 5 6
rect 4 5 5 6
rect 5 6 5 6
rect 12 13 5 6
rect 13 14 5 6
rect 14 15 5 6
rect 15 16 5 6
rect 16 17 5 6
rect 1 2 6 7
rect 2 3 6 7
rect 3 4 6 7
rect 4 5 6 7
rect 13 14 6 7
rect 14 15 6 7
rect 15 16 6 7
rect 1 2 7 8
rect 2 3 7 8
rect 3 4 7 8
rect 4 5 7 8
rect 5 6 7 8
rect 6 7 7 8
rect 7 8 7 8
rect 8 9 7 8
rect 12 13 7 8
rect 13 14 7 8
rect 14 15 7 8
rect 15 16 7 8
rect 16 17 7 8
rect 1 2 8 9
rect 2 3 8 9
rect 3 4 8 9
rect 4 5 8 9
rect 6 7 8 9
rect 7 8 8 9
rect 8 9 8 9
rect 9 10 8 9
rect 10 11 8 9
rect 11 12 8 9
rect 12 13 8 9
rect 13 14 8 9
rect 14 15 8 9
rect 1 2 9 10
rect 2 3 9 10
rect 3 4 9 10
rect 4 5 9 10
rect 6 7 9 10
rect 9 10 9 10
rect 11 12 9 10
rect 12 13 9 10
rect 13 14 9 10
rect 14 15 9 10
rect 1 2 10 11
rect 2 3 10 11
rect 4 5 10 11
rect 7 8 10 11
rect 8 9 10 11
rect 9 10 10 11
rect 13 14 10 11
rect 14 15 10 11
rect 1 2 11 12
rect 2 3 11 12
rect 3 4 11 12
rect 8 9 11 12
rect 11 12 11 12
rect 12 13 11 12
rect 13 14 11 12
rect 1 2 12 13
rect 2 3 12 13
rect 3 4 12 13
rect 8 9 12 13
rect 9 10 12 13
rect 2 3 13 14
rect 3 4 13 14
rect 7 8 13 14
rect 3 4 14 15
rect 7 8 14 15
rect 8 9 14 15
rect 9 10 14 15
rect 10 11 14 15
rect 3 4 15 16
rect 5 6 15 16
rect 1 2 16 17
rect 2 3 16 17
rect 6 7 16 17
rect 10 11 16 17
rect 11 12 16 17
`},
		{"peter2.nex", "###\n#\n####\n", "rect 1 4 1 2\nrect 1 2 2 3\nrect 1 5 3 4\n"},
		{"u.nex", "١ + ٢ + ... + ١٨ = 一百五十三", "1 + 2 + ... + 18 = 153"},
	} {
		cmd := exec.Command(nexBin, "-r", "-s", x.prog)
		cmd.Stdin = strings.NewReader(x.in)
		got, err := cmd.CombinedOutput()
		dieErr(t, err, x.prog+" "+string(got))
		if string(got) != x.out {
			t.Fatalf("program: %s\nwant %q, got %q", x.prog, x.out, string(got))
		}
	}
}

// To save time, we combine several test cases into a single nex program.
func TestGiantProgram(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "nex")
	dieErr(t, err, "TempDir")
	wd, err := os.Getwd()
	dieErr(t, err, "Getwd")
	dieErr(t, os.Chdir(tmpdir), "Chdir")
	defer func() {
		dieErr(t, os.Chdir(wd), "Chdir")
	}()
	defer func() {
		dieErr(t, os.RemoveAll(tmpdir), "RemoveAll")
	}()
	s := "package main\n"
	body := ""
	for i, x := range []struct {
		prog, in, out string
	}{
		// Test parentheses and $.
		{`
/[a-z]*/ <  { *lval += "[" }
  /a(($*|$$)($($)$$$))$($$$)*/ { *lval += "0" }
  /(e$|f$)/ { *lval += "1" }
  /(qux)*/  { *lval += "2" }
  /$/       { *lval += "." }
>           { *lval += "]" }
`, "a b c d e f g aaab aaaa eeeg fffe quxqux quxq quxe",
"[0][.][.][.][1][1][.][.][0][.][1][2][2][21]"},
		// Exercise ^ and rule precedence.
		{`
/[a-z]*/ <  { *lval += "[" }
  /((^*|^^)(^(^)^^^))^(^^^)*bar/ { *lval += "0" }
  /(^foo)*/ { *lval += "1" }
  /^fooo$/  { *lval += "2" }
  /^f(oo)*/ { *lval += "3" }
  /^foo*/   { *lval += "4" }
  /^/       { *lval += "." }
>           { *lval += "]" }
`, "foo bar foooo fooo fooooo fooof baz foofoo",
"[1][0][3][2][4][4][.][1]"},
		// Anchored empty matches.
		{`
/^/ { *lval += "BEGIN" }
/$/ { *lval += "END" }
`, "", "BEGIN"},

		{`
/$/ { *lval += "END" }
/^/ { *lval += "BEGIN" }
`, "", "END"},

		{`
/^$/ { *lval += "BOTH" }
/^/ { *lval += "BEGIN" }
/$/ { *lval += "END" }
`, "", "BOTH"},
		// Patterns like awk's BEGIN and END.
		{`
<          { *lval += "[" }
  /[0-9]*/ { *lval += "N" }
  /;/      { *lval += ";" }
  /./      { *lval += "." }
>          { *lval += "]\n" }
`, "abc 123 xyz;a1b2c3;42", "[....N....;.N.N.N;N]\n"},
		// A partial match regex has no effect on an immediately following match.
		{`
/abcd/ { *lval += "ABCD" }
/\n/   { *lval += "\n" }
`, "abcd\nbabcd\naabcd\nabcabcd\n", "ABCD\nABCD\nABCD\nABCD\n"},

		// Nested regex test. The simplistic parser means we must use commented
		// braces to balance out quoted braces.
		// Sprinkle in a couple of return statements to check Lex() saves stack
		// state correctly between calls.
		{`
/a[bcd]*e/ < { *lval += "[" }
  /a/        { *lval += "A" }
  /bcd/ <    { *lval += "(" }
  /c/        { *lval += "X"; return 1 }
  >          { *lval += ")" }
  /e/        { *lval += "E" }
  /ccc/ <    {
    *lval += "{"
    // }  [balance out the quoted left brace]
  }
  /./        { *lval += "?" }
  >          {
    // {  [balance out the quoted right brace]
    *lval += "}"
    return 2
  }
>            { *lval += "]" }
/\n/ { *lval += "\n" }
/./ { *lval += "." }
`, "abcdeabcabcdabcdddcccbbbcde", "[A(X)E].......[A(X){???}(X)E]"},

		// Exercise hyphens in character classes.
		{`
/[a-z-]*/ < { *lval += "[" }
  /[^-a-df-m]/ { *lval += "0" }
  /./       { *lval += "1" }
>           { *lval += "]" }
/\n/ { *lval += "\n" }
/./ { *lval += "." }
`, "-azb-ycx@d--w-e-", "[11011010].[1110101]"},

		// Overlapping character classes.
		{`
/[a-e]+[d-h]+/ { *lval += "0" }
/[m-n]+[k-p]+[^k-r]+[o-p]+/ { *lval += "1" }
/./ { *(*string)(lval) += yylex.Text() }
`, "abcdefghijmnopabcoq", "0ij1q"},
	} {
		id := fmt.Sprintf("%v", i)
		s += `import "./nex_test` + id + "\"\n"
		dieErr(t, os.Mkdir("nex_test"+id, 0777), "Mkdir")
		dieErr(t, ioutil.WriteFile(id+".nex", []byte(x.prog+`//
package nex_test`+id+`

type yySymType string

func Go() {
  x := NewLexer(bufio.NewReader(strings.NewReader(`+"`"+x.in+"`"+`)))
  lval := new(yySymType)
  for x.Lex(lval) != 0 { }
  s := string(*lval)
  if s != `+"`"+x.out+"`"+`{
    panic(`+"`"+x.prog+": want "+x.out+", got ` + s"+`)
  }
}
`), 0777), "WriteFile")
		_, err := exec.Command(nexBin, "-o", "nex_test"+id+"/tmp.go", id+".nex").CombinedOutput()
		dieErr(t, err, "nex: "+s)
		body += "nex_test" + id + ".Go()\n"
	}
	s += "func main() {\n" + body + "}\n"
	err = ioutil.WriteFile("tmp.go", []byte(s), 0777)
	dieErr(t, err, "WriteFile")
	output, err := exec.Command("go", "run", "tmp.go").CombinedOutput()
	dieErr(t, err, string(output))
}
