package nn
import ("bufio";"os")

type rule struct {
  b []byte
}

const count = 5
var a [count]*rule
func init() {
  newRule := func(s string) *rule {
    r := new(rule)
    r.b = make([]byte, len(s))
    copy(r.b, s)
    return r
  }
  a[0] = newRule("foo")
  a[1] = newRule("bar")
  a[2] = newRule("barr")
  a[3] = newRule(" ")
  a[4] = newRule("\n")
}

type ctx struct {
  done bool
  match, matchn, n int
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

func IsDone(p interface{}) bool {
  c := p.(*ctx)
  return c.done
}

func Iterate(p interface {}) int {
  c := p.(*ctx)
  if c.n == len(c.buf) {
    r,_,er := c.in.ReadRune()
    switch er {
    case nil: c.buf = append(c.buf, r)
    case os.EOF:
      c.done = true
      return getAction(c)
    default: panic(er.String())
    }
  }
  jammed := true
  r := c.buf[c.n]
  for i, x := range a {
    if -1 == c.state[i] { continue }
    if len(x.b) == c.state[i] { c.state[i] = -1; continue }
    if r == int(x.b[c.state[i]]) {
      jammed = false
      c.state[i]++
      if len(c.buf) == len(x.b) {
	if -1 == c.match || c.matchn < len(c.buf) || c.match > i {
	  c.match = i
	  c.matchn = len(c.buf)
	}
      }
    } else {
      c.state[i] = - 1
    }
  }
  if jammed {
    c.n = 0
    for i, _ := range c.state { c.state[i] = 0 }
    copy(c.buf, c.buf[c.matchn:])
    c.buf = c.buf[:len(c.buf) - c.matchn]
    return getAction(c)
  }
  c.n++
  return -1
}

func getAction(c *ctx) int {
  if -1 == c.match { panic("No match") }
  action := c.match
  c.match = -1
  return action
}
