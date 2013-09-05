package nex

import (
  "os/exec"
  "strings"
  "testing"
)

const nexBin = "./nex"

func TestBundledNexPrograms(t *testing.T) {
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
  } {
    cmd := exec.Command(nexBin, "-r", "-s", x.prog)
    cmd.Stdin = strings.NewReader(x.in)
    got, err := cmd.CombinedOutput()
    if err != nil {
      t.Fatalf("%s:\n", got, err)
    }
    if string(got) != x.out {
      t.Fatalf("want %q, got %q", x.out, string(got))
    }
  }
}
