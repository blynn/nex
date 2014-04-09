package main

import (
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

func TestWax(t *testing.T) {
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
	run("cp tacky.nex tacky.y tacky.go build.sh " + tmpdir)
	wd, err := os.Getwd()
	dieErr(t, err, "Getwd")
	dieErr(t, os.Chdir(tmpdir), "Chdir")
	defer func() {
		dieErr(t, os.Chdir(wd), "Chdir")
	}()
  dieErr(t, os.Setenv("NEXBIN", nexBin), "Setenv")
	run("./build.sh")
	cmd := exec.Command("./tacky")
	cmd.Stdin = strings.NewReader(`
ID10T {
  7 salary = $50000  // Initech salary.
  8a disinterest =  // Right-hand side of assignment can span multiple lines.
    $0.01  // Additions are implicit.
    $0.22
    $3.33
  21 other income = $305326.13  // Salami slicing.
  22 total income = 7 8a 21
  40 deduction = [ID10T, TPS Worksheet:10]  // Line from another form.
  43 taxable income = 22 - 40
  44 tax = $123.45 + 43 * 33% - $678.90  // Test precedence, percentages.
  99 = $99.50
  100 = $100.00
  101 = $100.49
}

ID10T, TPS Worksheet {
  1 bank error = [ID10T:21]
  // Divide by $2500 and round up.
  // Looks wacky because all computations are in cents, but it works.
  // However, in whole dollar mode, any rounding is likely undesirable.
  2bad = (1 + $2500.00 - $0.01) / $2500.00
  // The following is an example of a formula that is wrongly computed in
  // whole dollar mode.
  2badwrongifrounded = 1 * 2bad * 0.2%
  // The right way: multiply before potential whole dollar rounding.
  2 = 1 * ((1 + $2500.00 - $0.01) / $2500.00) * 0.2%  // Fractional percentage.
  // The line '0' is reserved. It always means $0.
  3 = clip(0 - $5)  // This should be 0.
  4 = clip($5 - 0)  // This should be $5.
  5 = max(3, 4)
  6 = min(5, 2)
  10 = 6
}

// Test rounding to dollars.
whole_dollars_only = 1

27B-6 {
  99 = [ID10T:99]
  100 = [ID10T:100]
  101 = [ID10T:101]
}
`)
	want := `ID10T
7 salary $50000.00
8a disinterest $3.56
21 other income $305326.13
22 total income $355329.69
40 deduction $5.00
43 taxable income $355324.69
44 tax $116701.70
99  $99.50
100  $100.00
101  $100.49

ID10T, TPS Worksheet
1 bank error $305326.13
2bad  $1.23
2badwrongifrounded  $75110.23
2  $75110.23
3  $0.00
4  $5.00
5  $5.00
6  $5.00
10  $5.00

27B-6
99  $100.00
100  $100.00
101  $100.00

`
	got, err := cmd.CombinedOutput()
	dieErr(t, err, "CombinedOutput")
	if want != string(got) {
		t.Fatalf("want %q, got %q", want, string(got))
	}
}
