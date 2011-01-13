package main
import ("bufio";"bytes";"os";"strings")
type rule struct {
  regex []byte
  code string
}
func main() {
  in := bufio.NewReader(os.Stdin)
  var r int
  done := false
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
    buf.Truncate(0)
    for {
      buf.WriteRune(r)
      read()
      if done { break }
      if strings.IndexRune(" \n\t\r", r) != -1 { break }
    }
    skipws()
    if done { panic("regex '" + buf.String() + "' lacks code") }
    x := new(rule)
    x.regex = make([]byte, len(buf.Bytes()))
    copy(x.regex, buf.Bytes())
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
  for i, x := range rules {
    println(i, string(x.regex), x.code)
  }
}
