// Some copy-and-paste from src/pkg/regexp; hopefully this means our regexes
// will be largely compatible.
package main
import ("bufio";"bytes";"os";"strings")
type rule struct {
  regex []int
  code string
}
type Error string

func (e Error) String() string { return string(e) }

var (
  ErrInternal            = Error("internal error")
  ErrUnmatchedLpar       = Error("unmatched '('")
  ErrUnmatchedRpar       = Error("unmatched ')'")
  ErrUnmatchedLbkt       = Error("unmatched '['")
  ErrUnmatchedRbkt       = Error("unmatched ']'")
  ErrBadRange            = Error("bad range in character class")
  ErrExtraneousBackslash = Error("extraneous backslash")
  ErrBareClosure         = Error("closure applies to nothing")
  ErrBadBackslash        = Error("illegal backslash escape")
)
func ispunct(c int) bool {
  for _, r := range "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~" {
    if c == r { return true }
  }
  return false
}
var escapes = []int("abfnrtv")
var escaped = []int("\a\b\f\n\r\t\v")
func escape(c int) int {
  for i, b := range escapes {
    if int(b) == c { return i }
  }
  return -1
}
type edge struct {
  kind int
  r int
  dst *node
}
type node struct {
  e []*edge
  n int
  accept bool
}
func f(s []int) {
  pos := 0
  n := 0
  newNode := func() *node {
    res := new(node)
    res.n = n
    res.e = make([]*edge, 0, 8)
    n++
    return res
  }
  newEdge := func(u, v *node) *edge {
    res := new(edge)
    res.dst = v
    u.e = append(u.e, res)
    return res
  }
  newWildEdge := func(u, v *node) *edge {
    res := newEdge(u, v)
    res.kind = 1
    return res
  }
  newRuneEdge := func(u, v *node, r int) *edge {
    res := newEdge(u, v)
    res.kind = 0
    res.r = r
    return res
  }
  newNilEdge := func(u, v *node) *edge {
    res := newEdge(u, v)
    res.kind = -1
    return res
  }
  nlpar := 0
  var pre func() (start, end *node)
  pterm := func() (start, end *node) {
    if len(s) == pos || s[pos] == '|' {
      end = newNode()
      start = end
      return
    }
    switch s[pos] {
    case '*','+': panic(ErrBareClosure)
    case ')':
      if 0 == nlpar { panic(ErrUnmatchedRpar) }
      end = newNode()
      start = end
      return
    case '(':
      nlpar++
      pos++
      start, end = pre()
      if len(s) == pos || ')' != s[pos] { panic(ErrUnmatchedLpar) }
    case '.':
      start = newNode()
      end = newNode()
      newWildEdge(start, end)
    default:
      c  := s[pos]
      if '\\' == c {
	pos++
	if len(s) == pos { panic(ErrExtraneousBackslash) }
	c = s[pos]
	switch {
	case ispunct(c):
	case escape(c) >= 0: c = escaped[escape(s[pos])]
	default: panic(ErrBadBackslash)
	}
      }
      start = newNode()
      end = newNode()
      newRuneEdge(start, end, c)
    }
    pos++
    return
  }
  pclosure := func() (start, end *node) {
    start, end = pterm()
    if start == end { return }
    if len(s) == pos { return }
    switch(s[pos]) {
    case '*':
      newNilEdge(end, start)
      nend := newNode()
      newNilEdge(end, nend)
      start = end
      end = nend
    default:
      return
    }
    pos++
    return
  }
  pcat := func() (start, end *node) {
    for {
      nstart, nend := pclosure()
      if start == nil { start, end = nstart, nend } else if nstart != nend {
	end.e = make([]*edge, len(nstart.e))
	copy(end.e, nstart.e)
	end = nend
      }
      if nstart == nend { return }
    }
    panic("unreachable")
  }
  pre = func() (start, end *node) {
    start,end = pcat()
    for {
      if len(s) == pos { return }
      if s[pos] != '|' { return }
      pos++;
      nstart,nend := pcat()
      tmp := newNode()
      newNilEdge(tmp, start)
      newNilEdge(tmp, nstart)
      start = tmp
      tmp = newNode()
      newNilEdge(end, tmp)
      newNilEdge(nend, tmp)
      end = tmp
    }
    panic("unreachable")
  }
  start, end := pre()
  end.accept = true

  var show func(*node)
  mark := make([]bool, n)
  show = func(u *node) {
    mark[u.n] = true
    print(u.n, ": ")
    for _,e := range u.e {
      print("(", e.kind, " ", e.r, ")")
      print(e.dst.n)
    }
    println()
    for _,e := range u.e {
      if !mark[e.dst.n] {
	show(e.dst)
      }
    }
  }
  show(start)
}
func main() {
  in := bufio.NewReader(os.Stdin)
  var r int
  done := false
  regex := make([]int, 0, 8)
  buf := bytes.NewBufferString("")
  read := func() {
    var er os.Error
    r,_,er = in.ReadRune()
    if er == os.EOF { done = true } else if er != nil { panic(er.String()) }
  }
  skipws := func() {
    for {
      read()
      if done { break }
      if strings.IndexRune(" \n\t\r", r) == -1 { break }
    }
  }
  var rules []*rule
  for !done {
    skipws()
    if done { break }
    regex = regex[0:0]
    for {
      regex = append(regex, r)
      read()
      if done { break }
      if strings.IndexRune(" \n\t\r", r) != -1 { break }
    }
    skipws()
    if done { panic("last pattern lacks action") }
    x := new(rule)
    x.regex = make([]int, len(regex))
    copy(x.regex, regex)
    if '{' != r { panic("expected {") }
    buf.Truncate(0)
    for {
      buf.WriteRune(r)
      read()
      if done { break }
      if '}' == r { break }
    }
    if done { panic("expected }") }
    buf.WriteRune(r)
    x.code = buf.String()
    rules = append(rules, x)
  }
  out := bufio.NewWriter(os.Stdout)
  for i, x := range rules {
    for _,v := range x.regex {
      out.WriteRune(v)
      out.Flush()
    }
    println(i, x.regex, x.code)
    f(x.regex)
  }
}
