// Substantial copy-and-paste from src/pkg/regexp.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
)
import (
	"go/parser"
	"go/printer"
	"go/token"
)

type rule struct {
	regex             []rune
	code              string
	id, family, index int
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
  ErrExpectedLBrace      = Error("expected '{'")
  ErrUnmatchedLBrace     = Error("unmatched '{'")
  ErrUnexpectedEOF       = Error("unexpected EOF")
  ErrUnexpectedNewline   = Error("unexpected newline")
  ErrUnmatchedLAngle     = Error("unmatched '<'")
  ErrUnmatchedRAngle     = Error("unmatched '>'")
)

func ispunct(c rune) bool {
	for _, r := range "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~" {
		if c == r {
			return true
		}
	}
	return false
}

var escapes = []rune("abfnrtv")
var escaped = []rune("\a\b\f\n\r\t\v")

func escape(c rune) rune {
	for i, b := range escapes {
		if b == c {
			return escaped[i]
		}
	}
	return -1
}

const (
  kNil = iota
  kRune
  kClass
  kWild
)

type edge struct {
	kind   int    // Rune/Class/Wild/Nil.
	r      rune   // Rune for rune edges.
	lim    []rune // Pairs of limits for character class edges.
	negate bool   // True if the character class is negated.
	dst    *node  // Destination node.
}
type node struct {
	e      []*edge // Outedges.
	n      int     // Index number. Scoped to a family.
	accept bool    // True if this is an accepting state.
	set    []int   // The NFA nodes represented by a DFA node.
}

// Print a graph in DOT format given the start node.
//
//  $ dot -Tps input.dot -o output.ps
func writeDotGraph(outf *os.File, start *node, id string) {
  done := make(map[*node]bool)
  var show func(*node)
  show = func(u *node) {
    if u.accept {
      fmt.Fprintf(outf, "  %v[style=filled,color=green];\n", u.n)
    }
    done[u] = true
    for _, e := range u.e {
      // We use -1 to denote the dead end node in DFAs.
      if e.dst.n == -1 {
        continue;
      }
      label := ""
      runeToDot := func(r rune) string {
        if strconv.IsPrint(r) {
          return fmt.Sprintf("%v", string(r))
        }
        return fmt.Sprintf("U+%X", int(r))
      }
      switch e.kind {
      case kRune:
        label = fmt.Sprintf("[label=%q]", runeToDot(e.r))
      case kWild:
        label = "[color=blue]"
      case kClass:
        label = "[label=\"["
        if e.negate {
          label += "^"
        }
        for i := 0; i < len(e.lim); i += 2 {
          label += runeToDot(e.lim[i])
          if e.lim[i] != e.lim[i + 1] {
            label += "-" + runeToDot(e.lim[i + 1])
          }
        }
        label += "]\"]"
      }
      fmt.Fprintf(outf, "  %v -> %v%v;\n", u.n, e.dst.n, label)
    }
    for _, e := range u.e {
      if !done[e.dst] {
        show(e.dst)
      }
    }
  }
  fmt.Fprintf(outf, "digraph %v {\n  0[shape=box];\n", id)
  show(start)
  fmt.Fprintln(outf, "}")
}

func inClass(r rune, lim []rune) bool {
	for i := 0; i < len(lim); i += 2 {
		if lim[i] <= r && r <= lim[i+1] {
			return true
		}
	}
	return false
}

var dfadot, nfadot *os.File

func gen(out io.Writer, x *rule) {
	// End rule
	if -1 == x.index {
		fmt.Fprintf(out, "a[%d].endcase = %d\n", x.family, x.id)
		return
	}
	s := x.regex
	// Regex -> NFA
	// We cannot have our alphabet be all Unicode characters. Instead,
	// we compute an alphabet for each regex:
	//
	//   1. Singles: we add single runes used in the regex: any rune not in a
	//   range. These are held in `sing`.
  //
	//   2. Ranges: entire ranges become elements of the alphabet. If ranges in
	//   the same expression overlap, we break them up into non-overlapping
	//   ranges. The generated code checks singles before ranges, so there's no
	//   need to break up a range if it contains a single. These are maintained
	//   in sorted order in `lim`.
  //
	//   3. Wild: we add an element representing all other runes.
	//
	// e.g. the alphabet of /[0-9]*[Ee][2-5]*/ is sing: { E, e },
	// lim: { [0-1], [2-5], [6-9] } and the wild element.
	sing := make(map[rune]bool)
	var lim []rune
	var insertLimits func(l, r rune)
	// Insert a new range [l-r] into `lim`, breaking it up if it overlaps, and
	// discarding it if it coincides with an existing range. We keep `lim`
	// sorted.
	insertLimits = func(l, r rune) {
		var i int
		for i = 0; i < len(lim); i += 2 {
			if l <= lim[i+1] {
				break
			}
		}
		if len(lim) == i || r < lim[i] {
			lim = append(lim, 0, 0)
			copy(lim[i+2:], lim[i:])
			lim[i] = l
			lim[i+1] = r
			return
		}
		if l < lim[i] {
			lim = append(lim, 0, 0)
			copy(lim[i+2:], lim[i:])
			lim[i+1] = lim[i] - 1
			lim[i] = l
			insertLimits(lim[i], r)
			return
		}
		if l > lim[i] {
			lim = append(lim, 0, 0)
			copy(lim[i+2:], lim[i:])
			lim[i+1] = l - 1
			lim[i+2] = l
			insertLimits(l, r)
			return
		}
		// l == lim[i]
		if r == lim[i+1] {
			return
		}
		if r < lim[i+1] {
			lim = append(lim, 0, 0)
			copy(lim[i+2:], lim[i:])
			lim[i] = l
			lim[i+1] = r
			lim[i+2] = r + 1
			return
		}
		insertLimits(lim[i+1]+1, r)
	}
	pos := 0
	n := 0
	newNode := func() *node {
		res := new(node)
		res.n = n
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
		res.kind = kWild
		return res
	}
	newRuneEdge := func(u, v *node, r rune) *edge {
		res := newEdge(u, v)
		res.kind = kRune
		res.r = r
		sing[r] = true
		return res
	}
	newNilEdge := func(u, v *node) *edge {
		res := newEdge(u, v)
		res.kind = kNil
		return res
	}
	newClassEdge := func(u, v *node) *edge {
		res := newEdge(u, v)
		res.kind = kClass
		res.lim = make([]rune, 0, 2)
		return res
	}
	nlpar := 0
	maybeEscape := func() rune {
		c := s[pos]
		if '\\' == c {
			pos++
			if len(s) == pos {
				panic(ErrExtraneousBackslash)
			}
			c = s[pos]
			switch {
			case ispunct(c):
			case escape(c) >= 0:
				c = escape(s[pos])
			default:
				panic(ErrBadBackslash)
			}
		}
		return c
	}
	pcharclass := func() (start, end *node) {
		start, end = newNode(), newNode()
		e := newClassEdge(start, end)
		// Ranges consisting of a single element are a special case:
		singletonRange := func(c rune) {
			// 1. The edge-specific 'lim' field always expects endpoints in pairs,
			// so we must give 'c' as the beginning and the end of the range.
			e.lim = append(e.lim, c, c)
			// 2. Instead of updating the regex-wide 'lim' interval set, we add a singleton.
			sing[c] = true
		}
		if len(s) > pos && '^' == s[pos] {
			e.negate = true
			pos++
		}
		var left rune
		leftLive := false
		justSawDash := false
		first := true
		// Allow '-' at the beginning and end, and in ranges.
		for pos < len(s) && s[pos] != ']' {
			switch c := maybeEscape(); c {
			case '-':
			  if first {
					singletonRange('-')
					break
				}
				justSawDash = true
			default:
				if justSawDash {
					if !leftLive || left > c {
						panic(ErrBadRange)
					}
					e.lim = append(e.lim, left, c)
					if left == c {
						sing[c] = true
					} else {
						insertLimits(left, c)
					}
					leftLive = false
				} else {
					if leftLive {
						singletonRange(left)
					}
					left = c
					leftLive = true
				}
				justSawDash = false
			}
			first = false
			pos++
		}
		if leftLive {
			singletonRange(left)
		}
		if justSawDash {
			singletonRange('-')
		}
		return
	}
	var pre func() (start, end *node)
	pterm := func() (start, end *node) {
		if len(s) == pos || s[pos] == '|' {
			end = newNode()
			start = end
			return
		}
		switch s[pos] {
		case '*', '+', '?':
			panic(ErrBareClosure)
		case ')':
			if 0 == nlpar {
				panic(ErrUnmatchedRpar)
			}
			end = newNode()
			start = end
			return
		case '(':
			nlpar++
			pos++
			start, end = pre()
			if len(s) == pos || ')' != s[pos] {
				panic(ErrUnmatchedLpar)
			}
		case '.':
			start = newNode()
			end = newNode()
			newWildEdge(start, end)
		case ']':
			panic(ErrUnmatchedRbkt)
		case '[':
			pos++
			start, end = pcharclass()
			if len(s) == pos || ']' != s[pos] {
				panic(ErrUnmatchedLbkt)
			}
		default:
			start = newNode()
			end = newNode()
			newRuneEdge(start, end, maybeEscape())
		}
		pos++
		return
	}
	pclosure := func() (start, end *node) {
		start, end = pterm()
		if start == end {
			return
		}
		if len(s) == pos {
			return
		}
		switch s[pos] {
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
			if start == nil {
				start, end = nstart, nend
			} else if nstart != nend {
				end.e = make([]*edge, len(nstart.e))
				copy(end.e, nstart.e)
				end = nend
			}
			if nstart == nend {
				return
			}
		}
		panic("unreachable")
	}
	pre = func() (start, end *node) {
		start, end = pcat()
		for {
			if len(s) == pos {
				return
			}
			if s[pos] != '|' {
				return
			}
			pos++
			nstart, nend := pcat()
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
			for _, e := range u.e {
				if !mark[e.dst.n] {
					visit(e.dst)
				}
			}
		}
		visit(start)
		for _, v := range short {
			v.n = newn[v.n]
		}
	}
	n = len(short)

	if nfadot != nil {
    writeDotGraph(nfadot, start, fmt.Sprintf("NFA_%v", x.id))
  }

	// NFA -> DFA
	nilClose := func(st []bool) {
		mark := make([]bool, n)
		var do func(int)
		do = func(i int) {
			v := short[i]
			for _, e := range v.e {
				if e.kind == kNil && !mark[e.dst.n] {
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
	var todo []*node
	tab := make(map[string]*node)
	var buf []byte
	dfacount := 0
	{  // Construct the node of no return.
		for i := 0; i < n; i++ {
			buf = append(buf, '0')
		}
		tmp := new(node)
		tmp.n = -1
		tab[string(buf)] = tmp
	}
	newDFANode := func(st []bool) (res *node, found bool) {
		buf = nil
		accept := false
		for i, v := range st {
			if v {
				buf = append(buf, '1')
				accept = accept || short[i].accept
			} else {
				buf = append(buf, '0')
			}
		}
		res, found = tab[string(buf)]
		if !found {
			res = new(node)
			res.n = dfacount
			res.accept = accept
			dfacount++
			for i, v := range st {
				if v {
					res.set = append(res.set, i)
				}
			}
			tab[string(buf)] = res
		}
		return res, found
	}

	get := func(states []bool) *node {
		nilClose(states)
		node, old := newDFANode(states)
		if !old {
			todo = append(todo, node)
		}
		return node
	}
	states := make([]bool, n)
  // The DFA start state is the state representing the nil-closure of the start
  // node in the NFA. Recall it has index 0.
  states[0] = true
	dfastart := get(states)
	for len(todo) > 0 {
		v := todo[len(todo)-1]
		todo = todo[0 : len(todo)-1]
		// Singles.
		for r, _ := range sing {
			states := make([]bool, n)
			for _, i := range v.set {
				for _, e := range short[i].e {
					if e.kind == kRune && e.r == r {
						states[e.dst.n] = true
					} else if e.kind == kWild {
						states[e.dst.n] = true
					} else if e.kind == kClass && e.negate != inClass(r, e.lim) {
						states[e.dst.n] = true
					}
				}
			}
			newRuneEdge(v, get(states), r)
		}
		// Character ranges.
		for j := 0; j < len(lim); j += 2 {
			states := make([]bool, n)
			for _, i := range v.set {
				for _, e := range short[i].e {
					if e.kind == kWild {
						states[e.dst.n] = true
					} else if e.kind == kClass && e.negate != inClass(lim[j], e.lim) {
						states[e.dst.n] = true
					}
				}
			}
			e := newClassEdge(v, get(states))
			e.lim = append(e.lim, lim[j], lim[j+1])
		}
		// Wild.
		states := make([]bool, n)
		for _, i := range v.set {
			for _, e := range short[i].e {
				if e.kind == kWild || (e.kind == kClass && e.negate) {
					states[e.dst.n] = true
				}
			}
		}
		newWildEdge(v, get(states))
	}
	n = dfacount

	if dfadot != nil {
    writeDotGraph(dfadot, dfastart, fmt.Sprintf("DFA_%v", x.id))
  }
	// DFA -> Go
	// TODO: Literal arrays instead of a series of assignments.
	fmt.Fprintf(out, "{\nvar acc [%d]bool\nvar fun [%d]func(rune) int\n", n, n)
	for _, v := range tab {
		if -1 == v.n {
			continue
		}
		if v.accept {
			fmt.Fprintf(out, "acc[%d] = true\n", v.n)
		}
		fmt.Fprintf(out, "fun[%d] = func(r rune) int {\n", v.n)
		fmt.Fprintf(out, "  switch(r) {\n")
		for _, e := range v.e {
			m := e.dst.n
			if e.kind == kRune {
				fmt.Fprintf(out, "  case %d: return %d\n", e.r, m)
			}
		}
		fmt.Fprintf(out, "  default:\n    switch {\n")
		for _, e := range v.e {
			m := e.dst.n
			if e.kind == kClass {
				fmt.Fprintf(out, "    case %d <= r && r <= %d: return %d\n",
					e.lim[0], e.lim[1], m)
			} else if e.kind == kWild {
				fmt.Fprintf(out, "    default: return %d\n", m)
			}
		}
		fmt.Fprintf(out, "    }\n  }\n  panic(\"unreachable\")\n}\n")
	}
	fmt.Fprintf(out, "a%d[%d].acc = acc[:]\n", x.family, x.index)
	fmt.Fprintf(out, "a%d[%d].f = fun[:]\n", x.family, x.index)
	fmt.Fprintf(out, "a%d[%d].id = %d\n", x.family, x.index, x.id)
	fmt.Fprintf(out, "}\n")
}

var standalone, customError, autorun *bool

func writeLex(out *bufio.Writer, rules []*rule) {
	if !*customError {
		out.WriteString(`func (yylex Lexer) Error(e string) {
  panic(e)
}`)
	}
	out.WriteString(`
func (yylex Lexer) Lex(lval *yySymType) int {
  for !yylex.isDone() {
    switch yylex.nextAction() {
    case -1:`)
	for _, x := range rules {
		fmt.Fprintf(out, "\n    case %d:  //%s/\n", x.id, string(x.regex))
		out.WriteString(x.code)
	}
	out.WriteString("    }\n  }\n  return 0\n}\n")
}
func writeNNFun(out *bufio.Writer, rules []*rule) {
	out.WriteString(`func(yylex Lexer) {
  for !yylex.isDone() {
    switch yylex.nextAction() {
    case -1:`)
	for _, x := range rules {
		fmt.Fprintf(out, "\n    case %d:  //%s/\n", x.id, string(x.regex))
		out.WriteString(x.code)
	}
	out.WriteString("    }\n  }\n  }")
}
func process(output io.Writer, input io.Reader) {
	in := bufio.NewReader(input)
	out := bufio.NewWriter(output)
	var r rune
	var regex []rune
	read := func() bool {
		var err error
		r, _, err = in.ReadRune()
		if err == io.EOF {
			return true
		}
		if err != nil {
			panic(err)
		}
		return false
	}
	skipws := func() bool {
		for !read() {
			if strings.IndexRune(" \n\t\r", r) == -1 {
				return false
			}
		}
		return true
	}
	var rules []*rule
	usercode := false
	familyn := 1
	id := 0
	newRule := func(family, index int) *rule {
		x := new(rule)
		rules = append(rules, x)
		x.family = family
		x.id = id
		x.index = index
		id++
		return x
	}
	var buf []rune
	readCode := func() string {
		if '{' != r {
			panic(ErrExpectedLBrace)
		}
		buf = nil
		nesting := 1
		for {
			buf = append(buf, r)
			if read() {
				panic(ErrUnmatchedLBrace)
			}
			if '{' == r {
				nesting++
			}
			if '}' == r {
				nesting--
				if 0 == nesting {
					break
				}
			}
		}
		buf = append(buf, r)
		return string(buf)
	}
	var decls string
	var parse func(int)
	parse = func(family int) {
		rulen := 0
		declvar := func() {
			decls += fmt.Sprintf("var a%d [%d]dfa\n", family, rulen)
		}
		for {
			if skipws() {
				break
			}
			regex = nil
			if '>' == r {
				if 0 == family {
					panic(ErrUnmatchedRAngle)
				}
				x := newRule(family, -1)
				x.code = "yylex = yylex.pop()\n"
				declvar()
				if skipws() {
					panic(ErrUnexpectedEOF)
				}
				x.code += readCode()
				return
			}
			delim := r
			if read() {
				panic(ErrUnexpectedEOF)
			}
			for {
				if r == delim && (len(regex) == 0 || regex[len(regex)-1] != '\\') {
					break
				}
				if '\n' == r {
					panic(ErrUnexpectedNewline)
				}
				regex = append(regex, r)
				if read() {
					panic(ErrUnexpectedEOF)
				}
			}
			if "" == string(regex) {
				usercode = true
				break
			}
			if skipws() {
				panic("last pattern lacks action")
			}
			x := newRule(family, rulen)
			rulen++
			x.regex = make([]rune, len(regex))
			copy(x.regex, regex)
			nested := false
			if '<' == r {
				if skipws() {
					panic("'<' lacks action")
				}
				x.code = fmt.Sprintf("yylex = yylex.push(%d)\n", familyn)
				nested = true
			}
			x.code += readCode()
			if nested {
				familyn++
				parse(familyn - 1)
			}
		}
		if 0 != family {
			panic(ErrUnmatchedLAngle)
		}
		x := newRule(family, -1)
		x.code = "// [END]\n"
		declvar()
	}
	parse(0)

	if !usercode {
		return
	}

	buf = nil
	for done := skipws(); !done; done = read() {
		buf = append(buf, r)
	}
	fs := token.NewFileSet()
	t, err := parser.ParseFile(fs, "", string(buf), parser.ImportsOnly)
	if err != nil {
		panic(err)
	}
	printer.Fprint(out, fs, t)

	var file *token.File
	fs.Iterate(func(f *token.File) bool {
		file = f
		return true
	})

	for m := file.LineCount(); m > 1; m-- {
		i := 0
		for '\n' != buf[i] {
			i++
		}
		buf = buf[i+1:]
	}

  out.WriteString(`import ("bufio";"io";"strings")
type dfa struct {
  acc []bool  // Accepting states.
  f []func(rune) int  // Transitions.
  id int
}
type family struct {
  a []dfa
  endcase int
}
` + decls +
`var a []family
func init() {
`)

	fmt.Fprintf(out, "a = make([]family, %d)\n", familyn)
	for _, x := range rules {
		gen(out, x)
	}
	for i := 0; i < familyn; i++ {
		fmt.Fprintf(out, "a[%d].a = a%d[:]\n", i, i)
	}

	out.WriteString(`}
func getAction(c *frame) int {
  if -1 == c.matchi { return -1 }
  c.action = c.fam.a[c.matchi].id
  c.matchi = -1
  return c.action
}
type frame struct {
  atEOF bool
  matchi int  // Index of DFA with highest-precedence match so far; -1 means no match yet.
  matchn int  // Length of highest-precedence match so far.
  action, n int
  buf []rune
  text string
  in *bufio.Reader
  state []int
  fam family
}
func newFrame(in *bufio.Reader, index int) *frame {
  f := new(frame)
  f.buf = make([]rune, 0, 128)
  f.in = in
  f.matchi = -1
  f.fam = a[index]
  f.state = make([]int, len(f.fam.a))
  return f
}
type Lexer []*frame
func NewLexer(in io.Reader) Lexer {
  var stack []*frame
  stack = append(stack, newFrame(bufio.NewReader(in), 0))
  return stack
}
func (stack Lexer) isDone() bool {
  return 1 == len(stack) && stack[0].atEOF
}
func (stack Lexer) nextAction() int {
  c := stack[len(stack) - 1]
  for {
    if c.atEOF { return c.fam.endcase }
    if c.n == len(c.buf) {
      r,_,er := c.in.ReadRune()
      switch er {
      case nil: c.buf = append(c.buf, r)
      case io.EOF:
	c.atEOF = true
	if c.n > 0 {
	  c.text = string(c.buf)
	  return getAction(c)
	}
	return c.fam.endcase
      default: panic(er)
      }
    }
    jammed := true
    r := c.buf[c.n]
    for i, x := range c.fam.a {
      if -1 == c.state[i] { continue }
      c.state[i] = x.f[c.state[i]](r)
      if -1 == c.state[i] { continue }
      jammed = false
      if x.acc[c.state[i]] {
        // Higher precedence match? Since the DFAs are run in parallel, c.matchn is at most c.n + 1, so we skip length equality check for the 3rd condition.
        if -1 == c.matchi || c.matchn < c.n + 1 || c.matchi > i {
          c.matchi = i
          c.matchn = c.n + 1
        }
      }
    }
    if jammed {
      a := getAction(c)
      if -1 == a { c.matchn = c.n + 1 }
      c.n = 0
      for i, _ := range c.state { c.state[i] = 0 }
      c.text = string(c.buf[:c.matchn])
      copy(c.buf, c.buf[c.matchn:])
      c.buf = c.buf[:len(c.buf) - c.matchn]
      return a
    }
    c.n++
  }
  panic("unreachable")
}
func (stack Lexer) push(index int) Lexer {
  c := stack[len(stack) - 1]
  return append(stack,
      newFrame(bufio.NewReader(strings.NewReader(c.text)), index))
}
func (stack Lexer) pop() Lexer {
  return stack[:len(stack) - 1]
}
func (stack Lexer) Text() string {
  c := stack[len(stack) - 1]
  return c.text
}
`)
	if !*standalone {
		writeLex(out, rules)
		out.WriteString(string(buf))
		out.Flush()
		return
	}
	m := 0
	const funmac = "NN_FUN"
	for m < len(buf) {
		m++
		if funmac[:m] != string(buf[:m]) {
			out.WriteString(string(buf[:m]))
			buf = buf[m:]
			m = 0
		} else if funmac == string(buf[:m]) {
			writeNNFun(out, rules)
			buf = buf[m:]
			m = 0
		}
	}
	out.WriteString(string(buf))
	out.Flush()
}

func dieIf(cond bool, v ...interface{}) {
  if cond {
    fmt.Println(v...)
    os.Exit(1)
  }
}

func dieErr(err error, s string) {
  if err != nil {
    fmt.Printf("%v: %v", s, err)
    os.Exit(1)
  }
}

func createDotFile(filename string) *os.File {
  if filename == "" {
    return nil
  }
  dieIf(strings.HasSuffix(filename, ".nex"), "nex: DOT filename ends with .nex:", filename)
  file, err := os.Create(filename)
  dieErr(err, "Create")
  return file
}

func main() {
	standalone  = flag.Bool("s", false, `standalone code; NN_FUN macro substitution, no Lex() method`)
	customError = flag.Bool("e", false, `custom error func; no Error() method`)
	autorun     = flag.Bool("r", false, `run generated program`)
	nfadotFile := flag.String("nfadot", "", `show NFA graph in DOT format`)
	dfadotFile := flag.String("dfadot", "", `show DFA graph in DOT format`)
	flag.Parse()

  nfadot = createDotFile(*nfadotFile)
  dfadot = createDotFile(*dfadotFile)
  defer func() {
    if (nfadot != nil) {
      dieErr(nfadot.Close(), "Close")
    }
    if (dfadot != nil) {
      dieErr(dfadot.Close(), "Close")
    }
  }()
  infile, outfile := os.Stdin, os.Stdout
  var err error
	if flag.NArg() > 0 {
    dieIf(flag.NArg() > 1, "nex: extraneous arguments after", flag.Arg(0))
    dieIf(strings.HasSuffix(flag.Arg(0), ".go"), "nex: input filename ends with .go:", flag.Arg(0))
    basename := flag.Arg(0)
    n := strings.LastIndex(basename, ".")
    if n >= 0 {
      basename = basename[:n]
    }
    infile, err = os.Open(flag.Arg(0))
    dieIf(infile == nil, "nex:", err)
    defer infile.Close()
    if !*autorun {
      outfile, err = os.Create(basename + ".nn.go")
      dieIf(outfile == nil, "nex:", err)
      defer outfile.Close()
    }
  }
  if *autorun {
    tmpdir, err := ioutil.TempDir("", "nex")
    dieIf(err != nil, "tempdir:", err)
    defer func() {
      err = os.RemoveAll(tmpdir)
      dieIf(err != nil, "removeall %q: %s", tmpdir, err)
    }()
    outfile, err = os.Create(tmpdir + "/lets.go")
    dieIf(outfile == nil, "nex:", err)
    defer outfile.Close()
  }
	process(outfile, infile)
  if *autorun {
    c := exec.Command("go", "run", outfile.Name())
    c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
    err = c.Run()
    dieIf(err != nil, "go run: %s", err)
  }
}
