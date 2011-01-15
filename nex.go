// Some copy-and-paste from src/pkg/regexp; hopefully this means our regexes
// will be largely compatible.
package main
import ("bufio";"flag";"fmt";"io";"os";"strings")
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
  set []int  // For the NFA -> DFA conversion.
}
func gen(out io.Writer, rulei int, s []int) {
  // Regex -> NFA
  alph := make(map[int]bool)
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
    alph[r] = true
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
    case '*','+','?': panic(ErrBareClosure)
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
    case '+':
      newNilEdge(end, start)
      nend := newNode()
      newNilEdge(end, nend)
      end = nend
    case '?':
      newNilEdge(start, end)
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

  // Compute shortlist of nodes, as we may have discarded nodes left over
  // from parsing. Also, make short[0] the start node.
  short := make([]*node, 0, n)
  {
    var visit func(*node)
    mark := make([]bool, n)
    newn := make([]int, n)
    visit = func(u *node) {
      mark[u.n] = true
      newn[u.n] = len(short)
      short = append(short, u)
      for _,e := range u.e {
	if !mark[e.dst.n] {
	  visit(e.dst)
	}
      }
    }
    visit(start)
    for _,v := range short {
      v.n = newn[v.n]
    }
  }
  n = len(short)

/*
  {
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
  */

  // NFA -> DFA
  nilClose := func(st []bool) {
    mark := make([]bool, n)
    var do func(int)
    do = func(i int) {
      v := short[i]
      for _,e := range v.e {
	if -1 == e.kind && !mark[e.dst.n] {
	  st[e.dst.n] = true
	  do(e.dst.n)
	}
      }
    }
    for i := 0; i < n; i++ {
      if st[i] && !mark[i] {
	mark[i] = true
	do(i)
      }
    }
  }
  states := make([]bool, n)
  states[0] = true
  nilClose(states)

  todo := make([]*node, 0, n)
  tab := make(map[string]*node)
  buf := make([]byte, 0, 8)
  dfacount := 0
  {
    for i := 0; i < n; i++ {
      buf = append(buf, '0')
    }
    tmp := new(node)
    tmp.n = -1
    tab[string(buf)] = tmp
  }
  newDFANode := func(st []bool) (res *node, found bool) {
    buf = buf[:0]
    accept := false
    for i,v := range st {
      if v {
	buf = append(buf, '1')
	accept = accept || short[i].accept
      } else { buf = append(buf, '0') }
    }
    res, found = tab[string(buf)]
    if !found {
      res = new(node)
      res.set = make([]int, 0, 8)
      res.n = dfacount
      res.accept = accept
      dfacount++
      for i,v := range st {
	if v { res.set = append(res.set, i) }
      }
      tab[string(buf)] = res
    }
    return res, found
  }
  dfastart, _ := newDFANode(states)
  todo = append(todo, dfastart)
  for len(todo) > 0 {
    v := todo[len(todo)-1]
    todo = todo[0:len(todo)-1]
    for r,_ := range alph {
      states := make([]bool, n)
      for _,i := range v.set {
	for _,e := range short[i].e {
	  if e.kind == 0 && e.r == r {
	    states[e.dst.n] = true
	  } else if e.kind == 1 {
	    states[e.dst.n] = true
	  }
	}
      }
      nilClose(states)
      node, old := newDFANode(states)
      if !old {
	todo = append(todo, node)
      }
      newRuneEdge(v, node, r)
    }
    states := make([]bool, n)
    for _,i := range v.set {
      for _,e := range short[i].e {
	if e.kind == 1 {
	  states[e.dst.n] = true
	}
      }
    }
    nilClose(states)
    node, old := newDFANode(states)
    if !old {
      todo = append(todo, node)
    }
    newWildEdge(v, node)
  }
  n = dfacount

  // TODO: Literal arrays instead of a series of assignments.
  fmt.Fprintf(out, "{\nvar acc [%d]bool\nvar fun [%d]func(int) int\n", n, n)
  for _,v := range tab {
    if -1 == v.n { continue }
    if v.accept {
      fmt.Fprintf(out, "acc[%d] = true\n", v.n)
    }
    fmt.Fprintf(out, "fun[%d] = func(r int) int {\n", v.n)
    fmt.Fprintf(out, "  switch(r) {\n")
    for _,e := range v.e {
      m := e.dst.n
      if e.kind == 0 {
	fmt.Fprintf(out, "  case %d: return %d\n", e.r, m)
      } else {
	fmt.Fprintf(out, "  default: return %d\n", m)
      }
    }
    fmt.Fprintf(out, "  }\n  panic(\"unreachable\")\n}\n")
  }
  fmt.Fprintf(out, "a[%d].acc = acc[:]\n", rulei);
  fmt.Fprintf(out, "a[%d].f = fun[:]\n}\n", rulei);
}
func writeActions(out *bufio.Writer, rules []*rule, prefix string) {
  fmt.Fprintf(out, `func(in *bufio.Reader) {
  nnCtx := %s.NewContext(bufio.NewReader(os.Stdin))
  for {
    %s.Next(nnCtx)
    if %s.IsDone(nnCtx) { break }
    switch %s.Action(nnCtx) {`, prefix, prefix, prefix, prefix)
  for i, x := range rules {
    fmt.Fprintf(out, "\n    case %d:  // %s\n", i, string(x.regex))
    out.WriteString(x.code)
  }

  out.WriteString("    }\n  }\n}")
}
func process(in *bufio.Reader, out, outmain *bufio.Writer, name string) {
  var r int
  done := false
  regex := make([]int, 0, 8)
  buf := make([]int, 0, 8)
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
  usercode := false
  for !done {
    skipws()
    if done { break }
    regex = regex[:0]
    for {
      regex = append(regex, r)
      read()
      if done { break }
      if strings.IndexRune(" \n\t\r", r) != -1 { break }
    }
    if "%%" == string(regex) {
      usercode = true
      break
    }
    skipws()
    if done { panic("last pattern lacks action") }
    x := new(rule)
    x.regex = make([]int, len(regex))
    copy(x.regex, regex)
    if '{' != r { panic("expected {") }
    buf = buf[:0]
    for {
      buf = append(buf, r)
      read()
      if done { break }
      if '}' == r { break }
    }
    if done { panic("expected }") }
    buf = append(buf, r)
    x.code = string(buf)
    rules = append(rules, x)
  }
  fmt.Fprintf(out, "package %s\n", name)
  fmt.Fprintf(out, `import ("bufio";"os")
type dfa struct {
  acc []bool
  f []func(int) int
}
const count = %d
var a [count]dfa
func init() {
`, len(rules))

  for i, x := range rules { gen(out, i, x.regex) }

  out.WriteString(`}
func getAction(c *ctx) {
  if -1 == c.match { panic("No match") }
  c.action, c.match = c.match, -1
}
type ctx struct {
  atEOF bool
  action, match, matchn, n int
  buf []int
  in *bufio.Reader
  state [count]int
}
func NewContext(in *bufio.Reader) interface{} {
  c := new(ctx)
  c.buf = make([]int, 0, 128)
  c.in = in
  c.match = -1
  return c
}
func IsDone(p interface {}) bool { return -1 == p.(*ctx).action }
func Action(p interface {}) int { return p.(*ctx).action }
func Next(p interface {}) {
  c := p.(*ctx)
  c.action = -1
  for {
    if c.atEOF { return }
    if c.n == len(c.buf) {
      r,_,er := c.in.ReadRune()
      switch er {
      case nil: c.buf = append(c.buf, r)
      case os.EOF:
	c.atEOF = true
	if c.n > 0 { getAction(c) }
	return
      default: panic(er.String())
      }
    }
    jammed := true
    r := c.buf[c.n]
    for i, x := range a {
      if -1 == c.state[i] { continue }
      c.state[i] = x.f[c.state[i]](r)
      if -1 == c.state[i] { continue }
      jammed = false
      if x.acc[c.state[i]] {
	if -1 == c.match || c.matchn < len(c.buf) || c.match > i {
	  c.match = i
	  c.matchn = len(c.buf)
	}
      }
    }
    if jammed {
      c.n = 0
      for i, _ := range c.state { c.state[i] = 0 }
      copy(c.buf, c.buf[c.matchn:])
      c.buf = c.buf[:len(c.buf) - c.matchn]
      getAction(c)
      return
    }
    c.n++
  }
}
`)
  out.Flush()

  if !usercode { return }
  skipws()
  buf = buf[:0]
  const macro = "NN_FUN"
  for !done {
    buf = append(buf, r)
    if macro[0:len(buf)] != string(buf) {
      outmain.WriteString(string(buf))
      buf = buf[:0]
    } else if len(macro) == len(buf) {
      writeActions(outmain, rules, name)
      buf = buf[:0]
    }
    read()
  }
  outmain.WriteString(string(buf))
  outmain.Flush()
}

func main() {
  flag.Parse()
  if 0 == flag.NArg() {
    name := "_nn_"
    f,er := os.Open(name + ".go", os.O_WRONLY|os.O_CREATE, 0644)
    if f == nil {
      println("nex:", er.String())
      os.Exit(1)
    }
    outmain := bufio.NewWriter(os.Stdout)
    out := bufio.NewWriter(f)
    process(bufio.NewReader(os.Stdin), out, outmain, name)
    f.Close()
    return
  }
}
