package main
import ("bufio";"os")
type rule struct {
  b []byte
  i int
}
func main() {
  in := bufio.NewReader(os.Stdin)
  buf := make([]int, 0, 128)
  done := false
  var a [4]*rule
  newRule := func(s string) *rule {
    r := new(rule)
    r.b = make([]byte, len(s))
    copy(r.b, s)
    r.i = 0
    return r
  }
  a[0] = newRule("foo")
  a[1] = newRule("barr")
  a[2] = newRule(" ")
  a[3] = newRule("\n")
  for _, x := range a { x.i = 0 }
  match := -1
  matchn := 0
  n := 0
  run_match := func() {
    if -1 == match { panic("No match") }
    println("executing", match)
    copy(buf, buf[matchn:])
    buf = buf[:len(buf) - matchn]
    match = -1
    for _, x := range a { x.i = 0 }
    n = 0
  }
  for {
    r,_,er := in.ReadRune()
    if er == os.EOF { done = true } else if er != nil { panic(er.String()) }
    if done {
      run_match()
      break;
    }
    buf = append(buf, r)
    for n < len(buf) {
      r := buf[n]
      jammed := true
      for i, x := range a {
	if -1 == x.i { continue }
	if len(x.b) == x.i { x.i = -1; continue }
	if r == int(x.b[x.i]) {
	  jammed = false
	  x.i++
	  if len(buf) == len(x.b) {
	    if -1 == match || matchn < len(buf) || match > i {
	      match = i
	      matchn = len(buf)
	    }
	  }
	} else {
	  x.i = - 1
	}
      }
      if jammed { run_match() } else { n++ }
    }
  }
}
