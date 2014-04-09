// Tacky.
//
// See README.

package main

import (
  "fmt"
  "os"
  "strconv"
  "strings"
)

type Expr struct {
  op string
  s string
  kid []*Expr
}

func (e *Expr) AddKid(kid *Expr) {
  e.kid = append(e.kid, kid)
}

func NewExpr(op, s string) *Expr {
  return &Expr{op, s, nil}
}

func NewOp(op string, a *Expr, b *Expr) *Expr {
  return &Expr{op, "", []*Expr{a, b}}
}

func NewFun(op string, args *Expr) *Expr {
  s := strings.TrimRight(op, "(")
  e := NewExpr(s, s)
  e.kid = args.kid
  return e
}

type formLine struct {
  id string
  desc string
  expr *Expr
  cached bool
  n int64
}

type form struct {
  id string
  lines map[string]*formLine
  sortedIds []string
  wholeDollarsOnly bool
}

type Tacky struct {
  forms map[string]*form
  curForm *form
  curLine string
  curFlag string
  sortedIds []string
  wholeDollarsOnly bool
}

func (tax *Tacky) Expr(e *Expr) {
  if tax.curLine == "" {
    if tax.curFlag == "" {
      panic("assignment expected")
    }
    if e.op != "ID" {
      panic("flag must be 0 or 1")
    }
    switch e.s {
    case "1":
      tax.wholeDollarsOnly = true
    case "0":
      tax.wholeDollarsOnly = false
    default:
      panic("flag must be 0 or 1")
    }
    tax.curFlag = ""
    return
  }
  form := tax.curForm
  line, ok := form.lines[tax.curLine]
  if !ok {
    panic("BUG: missing line " + tax.curLine)
  }
  if line.expr == nil {
    line.expr = e
    return
  }
  line.expr = NewOp("+", line.expr, e)
}

func (tax *Tacky) Assign(s string) {
  form := tax.curForm
  s = strings.TrimRight(s, "=")
  v := strings.SplitN(s, " ", 2)
  id := v[0]
  if form == nil {
    if id != "whole_dollars_only" {
      panic("no such flag: " + id)
    }
    tax.curFlag = id
    return
  }
  desc := ""
  if len(v) == 2 {
    desc = v[1]
  }
  if _, ok := form.lines[id]; ok {
    panic("duplicate line definition: " + id)
  }
  form.lines[id] = &formLine{id, strings.TrimSpace(desc), nil, false, 0}
  form.sortedIds = append(form.sortedIds, id)
  tax.curLine = id
}

func (tax *Tacky) BeginForm(line string) {
  if tax.curForm != nil {
    panic("nested form")
  }
  id := strings.TrimSpace(strings.TrimRight(line, "{"))
  if _, ok := tax.forms[id]; ok {
    panic("duplicate form: " + id)
  }
  tax.curForm = &form{id, make(map[string]*formLine), nil, tax.wholeDollarsOnly}
  tax.curLine = ""
  tax.sortedIds = append(tax.sortedIds, id)
  tax.forms[id] = tax.curForm
}

func (tax *Tacky) EndForm() {
  tax.curForm = nil
  tax.curLine = ""
}

func atoi(s string) int64 {
  n, e := strconv.Atoi(s)
  if e != nil {
    panic(e)
  }
  return int64(n)
}

func clip(a int64) int64 {
  if a < 0 {
    return 0
  }
  return a
}

func min(a, b int64) int64 {
  if a < b {
    return a
  }
  return b
}

func max(a, b int64) int64 {
  if a > b {
    return a
  }
  return b
}

func (tax *Tacky) Eval(e *Expr) int64 {
  if e == nil {  // Undefined.
    return 0
  }
  switch e.op {
  case "$":
    s := strings.TrimLeft(e.s, "$")
    v := strings.Split(s, ".")
    if len(v) == 2 {
      return atoi(v[0]) * 100 + atoi(v[1])
    }
    if len(v) != 1 {
      panic("bad money: " + e.s)
    }
    return atoi(s) * 100
  case "ID":
    if e.s == "0" {
      return 0
    }
    return tax.Get(e.s)
  case "+":
    return tax.Eval(e.kid[0]) + tax.Eval(e.kid[1])
  case "-":
    return tax.Eval(e.kid[0]) - tax.Eval(e.kid[1])
  case "*":
    if e.kid[1].op == "%" {
      v := strings.Split(strings.TrimRight(e.kid[1].s, "%"), ".")
      p := atoi(v[0])
      q := int64(100)
      if len(v) == 2 {
        for _ = range v[1] {
          p *= 10
          q *= 10
        }
        p += atoi(v[1])
      } else if len(v) != 1 {
        panic("malformed percentage")
      }
      return (tax.Eval(e.kid[0]) * p + q / 2) / q
    }
    return tax.Eval(e.kid[0]) * tax.Eval(e.kid[1])
  case "/":
    return tax.Eval(e.kid[0]) / tax.Eval(e.kid[1])
  case "XREF":
    v := strings.Split(e.s, ":")
    return tax.XGet(strings.TrimLeft(v[0], "["), strings.TrimRight(v[1], "]"))
  case "clip":
    if len(e.kid) != 1 {
      panic("bad arg count")
    }
    return clip(tax.Eval(e.kid[0]))
  case "min":
    if len(e.kid) != 2 {
      panic("bad arg count")
    }
    return min(tax.Eval(e.kid[0]), tax.Eval(e.kid[1]))
  case "max":
    if len(e.kid) != 2 {
      panic("bad arg count")
    }
    return max(tax.Eval(e.kid[0]), tax.Eval(e.kid[1]))
  }
  panic("no such op: " + e.op)
}

func (tax *Tacky) Get(id string) int64 {
  line, ok := tax.curForm.lines[id]
  if !ok {
    panic("no such line: " + id)
  }
  if line.cached {
    return line.n
  }
  line.cached = true
  line.n = tax.Eval(line.expr)
  if tax.curForm.wholeDollarsOnly {
    line.n = (line.n + 50) / 100 * 100
  }
  return line.n
}

func (tax *Tacky) XGet(id, line string) int64 {
  form, ok := tax.forms[id]
  if !ok {
    panic("no such form: " + id)
  }
  orig := tax.curForm
  tax.curForm = form
  n := tax.Get(line)
  tax.curForm = orig
  return n
}

func dollarize(cents int64) string {
  if cents < 0 {
    return "(" + dollarize(-cents) + ")"
  }
  return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}

func main() {
  tax := new(Tacky)
  tax.forms = make(map[string]*form)
  yyParse(NewLexerWithInit(os.Stdin, func(y *Lexer) { y.p = tax }))

  for _, id := range tax.sortedIds {
    f := tax.forms[id]
    tax.curForm = f
    fmt.Println(f.id)
    for _, id := range f.sortedIds {
      fmt.Println(id, f.lines[id].desc, dollarize(tax.Get(id)))
    }
    fmt.Println()
  }
}
