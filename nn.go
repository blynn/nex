package nn
import ("bufio";"os")

type dfa struct {
  acc []bool
  f []func(int) int
}
const count = 2
var a [count]dfa
func init() {
{
var acc [2]bool
var fun [2]func(int) int
acc[1] = true
fun[1] = func(r int) int {
  switch(r) {
  case 10: return -1
  default: return -1
  }
  panic("unreachable")
}
fun[0] = func(r int) int {
  switch(r) {
  case 10: return 1
  default: return -1
  }
  panic("unreachable")
}
a[0].acc = acc[:]
a[0].f = fun[:]
}
{
var acc [2]bool
var fun [2]func(int) int
acc[1] = true
fun[1] = func(r int) int {
  switch(r) {
  default: return -1
  }
  panic("unreachable")
}
fun[0] = func(r int) int {
  switch(r) {
  default: return 1
  }
  panic("unreachable")
}
a[1].acc = acc[:]
a[1].f = fun[:]
}
}
func getAction(c *ctx) int {
  if -1 == c.match { panic("No match") }
  action := c.match
  c.match = -1
  return action
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
func Next(p interface {}) (bool, int) {
  c := p.(*ctx)
  for {
    if c.done { return true, -1 }
    if c.n == len(c.buf) {
      r,_,er := c.in.ReadRune()
      switch er {
      case nil: c.buf = append(c.buf, r)
      case os.EOF:
	c.done = true
	return false, getAction(c)
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
      return false, getAction(c)
    }
    c.n++
  }
}
