// Substantial copy-and-paste from src/pkg/regexp.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)
import (
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
)

type rule struct {
	regex     []rune
	code      string
	startCode string
	endCode   string
	kid       []*rule
	id        string
}

var (
	ErrInternal            = errors.New("internal error")
	ErrUnmatchedLpar       = errors.New("unmatched '('")
	ErrUnmatchedRpar       = errors.New("unmatched ')'")
	ErrUnmatchedLbkt       = errors.New("unmatched '['")
	ErrUnmatchedRbkt       = errors.New("unmatched ']'")
	ErrBadRange            = errors.New("bad range in character class")
	ErrExtraneousBackslash = errors.New("extraneous backslash")
	ErrBareClosure         = errors.New("closure applies to nothing")
	ErrBadBackslash        = errors.New("illegal backslash escape")
	ErrExpectedLBrace      = errors.New("expected '{'")
	ErrUnmatchedLBrace     = errors.New("unmatched '{'")
	ErrUnexpectedEOF       = errors.New("unexpected EOF")
	ErrUnexpectedNewline   = errors.New("unexpected newline")
	ErrUnexpectedLAngle    = errors.New("unexpected '<'")
	ErrUnmatchedLAngle     = errors.New("unmatched '<'")
	ErrUnmatchedRAngle     = errors.New("unmatched '>'")
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
	kStart
	kEnd
)

type edge struct {
	kind   int    // Rune/Class/Wild/Nil.
	r      rune   // Rune for rune edges.
	lim    []rune // Pairs of limits for character class edges.
	negate bool   // True if the character class is negated.
	dst    *node  // Destination node.
}
type node struct {
	e      edges // Outedges.
	n      int   // Index number. Scoped to a family.
	accept bool  // True if this is an accepting state.
	set    []int // The NFA nodes represented by a DFA node.
}

type edges []*edge

func (e edges) Len() int {
	return len(e)
}
func (e edges) Less(i, j int) bool {
	return e[i].r < e[j].r
}

func (e edges) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

type RuneSlice []rune

func (p RuneSlice) Len() int           { return len(p) }
func (p RuneSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p RuneSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

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
				continue
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
					if e.lim[i] != e.lim[i+1] {
						label += "-" + runeToDot(e.lim[i+1])
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

func gen(out *bufio.Writer, x *rule) {
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
		sort.Sort(u.e)
		return res
	}
	newStartEdge := func(u, v *node) *edge {
		res := newEdge(u, v)
		res.kind = kStart
		return res
	}
	newEndEdge := func(u, v *node) *edge {
		res := newEdge(u, v)
		res.kind = kEnd
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
	isNested := false
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
			if !isNested {
				panic(ErrUnmatchedRpar)
			}
			end = newNode()
			start = end
			return
		case '(':
			pos++
			oldIsNested := isNested
			isNested = true
			start, end = pre()
			isNested = oldIsNested
			if len(s) == pos || ')' != s[pos] {
				panic(ErrUnmatchedLpar)
			}
		case '.':
			start, end = newNode(), newNode()
			newWildEdge(start, end)
		case '^':
			start, end = newNode(), newNode()
			newStartEdge(start, end)
		case '$':
			start, end = newNode(), newNode()
			newEndEdge(start, end)
		case ']':
			panic(ErrUnmatchedRbkt)
		case '[':
			pos++
			start, end = pcharclass()
			if len(s) == pos || ']' != s[pos] {
				panic(ErrUnmatchedLbkt)
			}
		default:
			start, end = newNode(), newNode()
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
			start, end = end, nend
		case '+':
			newNilEdge(end, start)
			nend := newNode()
			newNilEdge(end, nend)
			end = nend
		case '?':
                        nstart := newNode()
			newNilEdge(nstart, start)
                        start = nstart
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
		for pos < len(s) && s[pos] != ')' {
			if s[pos] != '|' {
				panic(ErrInternal)
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
		return
	}
	start, end := pre()
	end.accept = true

	// Compute shortlist of nodes (reachable nodes), as we may have discarded
	// nodes left over from parsing. Also, make short[0] the start node.
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
		writeDotGraph(nfadot, start, "NFA_"+x.id)
	}

	// NFA -> DFA
	nilClose := func(st []bool) {
		visited := make([]bool, n)
		var do func(int)
		do = func(i int) {
                        visited[i] = true
			v := short[i]
			for _, e := range v.e {
				if e.kind == kNil && !visited[e.dst.n] {
					st[e.dst.n] = true
					do(e.dst.n)
				}
			}
		}
		for i := 0; i < n; i++ {
			if st[i] && !visited[i] {
				do(i)
			}
		}
	}
	var todo []*node
	tab := make(map[string]*node)
	var buf []byte
	dfacount := 0
	{ // Construct the node of no return.
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
	getcb := func(v *node, cb func(*edge) bool) *node {
		states := make([]bool, n)
		for _, i := range v.set {
			for _, e := range short[i].e {
				if cb(e) {
					states[e.dst.n] = true
				}
			}
		}
		return get(states)
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
		var runes []rune
		for r, _ := range sing {
			runes = append(runes, r)
		}
		sort.Sort(RuneSlice(runes))
		for _, r := range runes {
			newRuneEdge(v, getcb(v, func(e *edge) bool {
				return e.kind == kRune && e.r == r ||
					e.kind == kWild ||
					e.kind == kClass && e.negate != inClass(r, e.lim)
			}), r)
		}
		// Character ranges.
		for j := 0; j < len(lim); j += 2 {
			e := newClassEdge(v, getcb(v, func(e *edge) bool {
				return e.kind == kWild ||
					e.kind == kClass && e.negate != inClass(lim[j], e.lim)
			}))

			e.lim = append(e.lim, lim[j], lim[j+1])
		}
		// Wild.
		newWildEdge(v, getcb(v, func(e *edge) bool {
			return e.kind == kWild || (e.kind == kClass && e.negate)
		}))
		// ^ and $.
		newStartEdge(v, getcb(v, func(e *edge) bool { return e.kind == kStart }))
		newEndEdge(v, getcb(v, func(e *edge) bool { return e.kind == kEnd }))
	}
	n = dfacount

	if dfadot != nil {
		writeDotGraph(dfadot, dfastart, "DFA_"+x.id)
	}
	// DFA -> Go
	sorted := make([]*node, n)
	for _, v := range tab {
		if -1 != v.n {
			sorted[v.n] = v
		}
	}

	fmt.Fprintf(out, "\n// %v\n", string(x.regex))
	for i, v := range sorted {
		if i == 0 {
			out.WriteString("{[]bool{")
		} else {
			out.WriteString(", ")
		}
		if v.accept {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	}
	out.WriteString("}, []func(rune) int{  // Transitions\n")
	for _, v := range sorted {
		out.WriteString("func(r rune) int {\n")
		var runeCases, classCases string
		var wildDest int
		for _, e := range v.e {
			m := e.dst.n
			switch e.kind {
			case kRune:
				runeCases += fmt.Sprintf("\t\tcase %d: return %d\n", e.r, m)
			case kClass:
				classCases += fmt.Sprintf("\t\tcase %d <= r && r <= %d: return %d\n",
					e.lim[0], e.lim[1], m)
			case kWild:
				wildDest = m
			}
		}
		if runeCases != "" {
			out.WriteString("\tswitch(r) {\n" + runeCases + "\t}\n")
		}
		if classCases != "" {
			out.WriteString("\tswitch {\n" + classCases + "\t}\n")
		}
		fmt.Fprintf(out, "\treturn %v\n},\n", wildDest)
	}
	out.WriteString("}, []int{  /* Start-of-input transitions */ ")
	for _, v := range sorted {
		s := " -1,"
		for _, e := range v.e {
			if e.kind == kStart {
				s = fmt.Sprintf(" %d,", e.dst.n)
				break
			}
		}
		out.WriteString(s)
	}
	out.WriteString("}, []int{  /* End-of-input transitions */ ")
	for _, v := range sorted {
		s := " -1,"
		for _, e := range v.e {
			if e.kind == kEnd {
				s = fmt.Sprintf(" %d,", e.dst.n)
				break
			}
		}
		out.WriteString(s)
	}
	out.WriteString("},")
	if len(x.kid) == 0 {
		out.WriteString("nil")
	} else {
		out.WriteString("[]dfa{")
		for _, kid := range x.kid {
			gen(out, kid)
		}
		out.WriteString("}")
	}
	out.WriteString("},\n")
}

func writeFamily(out *bufio.Writer, node *rule, lvl int) {
	tab := func() {
		for i := 0; i <= lvl; i++ {
			out.WriteByte('\t')
		}
	}
	if node.startCode != "" {
		tab()
		prefixReplacer.WriteString(out, "if !yylex.stale {\n")
		tab()
		out.WriteString("\t" + node.startCode + "\n")
		tab()
		out.WriteString("}\n")
	}
	tab()
	fmt.Fprintf(out, "OUTER%s%d:\n", node.id, lvl)
	tab()
	prefixReplacer.WriteString(out,
		fmt.Sprintf("for { switch yylex.next(%v) {\n", lvl))
	for i, x := range node.kid {
		tab()
		fmt.Fprintf(out, "\tcase %d:\n", i)
		lvl++
		if x.kid != nil {
			writeFamily(out, x, lvl)
		} else {
			tab()
			out.WriteString("\t" + x.code + "\n")
		}
		lvl--
	}
	tab()
	out.WriteString("\tdefault:\n")
	tab()
	fmt.Fprintf(out, "\t\t break OUTER%s%d\n", node.id, lvl)
	tab()
	out.WriteString("\t}\n")
	tab()
	out.WriteString("\tcontinue\n")
	tab()
	out.WriteString("}\n")
	tab()
	prefixReplacer.WriteString(out, "yylex.pop()\n")
	tab()
	out.WriteString(node.endCode + "\n")
}

var lexertext = `import ("bufio";"io";"strings")
type frame struct {
  i int
  s string
  line, column int
}
type Lexer struct {
  // The lexer runs in its own goroutine, and communicates via channel 'ch'.
  ch chan frame
  ch_stop chan bool
  // We record the level of nesting because the action could return, and a
  // subsequent call expects to pick up where it left off. In other words,
  // we're simulating a coroutine.
  // TODO: Support a channel-based variant that compatible with Go's yacc.
  stack []frame
  stale bool

  // The 'l' and 'c' fields were added for
  // https://github.com/wagerlabs/docker/blob/65694e801a7b80930961d70c69cba9f2465459be/buildfile.nex
  // Since then, I introduced the built-in Line() and Column() functions.
  l, c int

  parseResult interface{}

  // The following line makes it easy for scripts to insert fields in the
  // generated code.
  // [NEX_END_OF_LEXER_STRUCT]
}

// NewLexerWithInit creates a new Lexer object, runs the given callback on it,
// then returns it.
func NewLexerWithInit(in io.Reader, initFun func(*Lexer)) *Lexer {
  yylex := new(Lexer)
  if initFun != nil {
    initFun(yylex)
  }
  yylex.ch = make(chan frame)
  yylex.ch_stop = make(chan bool, 1)
  var scan func(in *bufio.Reader, ch chan frame, ch_stop chan bool, family []dfa, line, column int) 
  scan = func(in *bufio.Reader, ch chan frame, ch_stop chan bool, family []dfa, line, column int) {
    // Index of DFA and length of highest-precedence match so far.
    matchi, matchn := 0, -1
    var buf []rune
    n := 0
    checkAccept := func(i int, st int) bool {
      // Higher precedence match? DFAs are run in parallel, so matchn is at most len(buf), hence we may omit the length equality check.
      if family[i].acc[st] && (matchn < n || matchi > i) {
        matchi, matchn = i, n
        return true
      }
      return false
    }
    var state [][2]int
    for i := 0; i < len(family); i++ {
      mark := make([]bool, len(family[i].startf))
      // Every DFA starts at state 0.
      st := 0
      for {
        state = append(state, [2]int{i, st})
        mark[st] = true
        // As we're at the start of input, follow all ^ transitions and append to our list of start states.
        st = family[i].startf[st]
        if -1 == st || mark[st] { break }
        // We only check for a match after at least one transition.
        checkAccept(i, st)
      }
    }
    atEOF := false
    stopped := false
    for {
      if n == len(buf) && !atEOF {
        r,_,err := in.ReadRune()
        switch err {
        case io.EOF: atEOF = true
        case nil:    buf = append(buf, r)
        default:     panic(err)
        }
      }
      if !atEOF {
        r := buf[n]
        n++
        var nextState [][2]int
        for _, x := range state {
          x[1] = family[x[0]].f[x[1]](r)
          if -1 == x[1] { continue }
          nextState = append(nextState, x)
          checkAccept(x[0], x[1])
        }
        state = nextState
      } else {
dollar:  // Handle $.
        for _, x := range state {
          mark := make([]bool, len(family[x[0]].endf))
          for {
            mark[x[1]] = true
            x[1] = family[x[0]].endf[x[1]]
            if -1 == x[1] || mark[x[1]] { break }
            if checkAccept(x[0], x[1]) {
              // Unlike before, we can break off the search. Now that we're at the end, there's no need to maintain the state of each DFA.
              break dollar
            }
          }
        }
        state = nil
      }

      if state == nil {
        lcUpdate := func(r rune) {
          if r == '\n' {
            line++
            column = 0
          } else {
            column++
          }
        }
        // All DFAs stuck. Return last match if it exists, otherwise advance by one rune and restart all DFAs.
        if matchn == -1 {
          if len(buf) == 0 {  // This can only happen at the end of input.
            break
          }
          lcUpdate(buf[0])
          buf = buf[1:]
        } else {
          text := string(buf[:matchn])
          buf = buf[matchn:]
          matchn = -1
          for {
            sent := false
            select {
              case ch <- frame{matchi, text, line, column}: {
                sent = true
              }
              case stopped = <- ch_stop: {
              }
              default: {
                // nothing
              }
            }
            if stopped||sent {
              break
            }
          }
          if stopped {
            break
          }
          if len(family[matchi].nest) > 0 {
            scan(bufio.NewReader(strings.NewReader(text)), ch, ch_stop, family[matchi].nest, line, column)
          }
          if atEOF {
            break
          }
          for _, r := range text {
            lcUpdate(r)
          }
        }
        n = 0
        for i := 0; i < len(family); i++ {
          state = append(state, [2]int{i, 0})
        }
      }
    }
    ch <- frame{-1, "", line, column}
  }
  go scan(bufio.NewReader(in), yylex.ch, yylex.ch_stop, dfas, 0, 0)
  return yylex
}

type dfa struct {
  acc []bool  // Accepting states.
  f []func(rune) int  // Transitions.
  startf, endf []int  // Transitions at start and end of input.
  nest []dfa
}

var dfas = []dfa{`

var lexeroutro = `}

func NewLexer(in io.Reader) *Lexer {
  return NewLexerWithInit(in, nil)
}

func (yyLex *Lexer) Stop() {
  yyLex.ch_stop <- true
}

// Text returns the matched text.
func (yylex *Lexer) Text() string {
  return yylex.stack[len(yylex.stack) - 1].s
}

// Line returns the current line number.
// The first line is 0.
func (yylex *Lexer) Line() int {
  if len(yylex.stack) == 0 {
    return 0
  }
  return yylex.stack[len(yylex.stack) - 1].line
}

// Column returns the current column number.
// The first column is 0.
func (yylex *Lexer) Column() int {
  if len(yylex.stack) == 0 {
    return 0
  }
  return yylex.stack[len(yylex.stack) - 1].column
}

func (yylex *Lexer) next(lvl int) int {
  if lvl == len(yylex.stack) {
    l, c := 0, 0
    if lvl > 0 {
      l, c = yylex.stack[lvl - 1].line, yylex.stack[lvl - 1].column
    }
    yylex.stack = append(yylex.stack, frame{0, "", l, c})
  }
  if lvl == len(yylex.stack) - 1 {
    p := &yylex.stack[lvl]
    *p = <-yylex.ch
    yylex.stale = false
  } else {
    yylex.stale = true
  }
  return yylex.stack[lvl].i
}
func (yylex *Lexer) pop() {
  yylex.stack = yylex.stack[:len(yylex.stack) - 1]
}
`

func writeLex(out *bufio.Writer, root rule) {
	if !customError {
		// TODO: I can't remember what this was for!
		prefixReplacer.WriteString(out, `func (yylex Lexer) Error(e string) {
  panic(e)
}`)
	}
	prefixReplacer.WriteString(out, `
// Lex runs the lexer. Always returns 0.
// When the -s option is given, this function is not generated;
// instead, the NN_FUN macro runs the lexer.
func (yylex *Lexer) Lex(lval *yySymType) int {
`)
	writeFamily(out, &root, 0)
	out.WriteString("\treturn 0\n}\n")
}
func writeNNFun(out *bufio.Writer, root rule) {
	prefixReplacer.WriteString(out, "func(yylex *Lexer) {\n")
	writeFamily(out, &root, 0)
	out.WriteString("}")
}
func process(output io.Writer, input io.Reader) error {
	lineno := 1
	in := bufio.NewReader(input)
	out := bufio.NewWriter(output)
	var r rune
	read := func() bool {
		var err error
		r, _, err = in.ReadRune()
		if err == io.EOF {
			return true
		}
		if err != nil {
			panic(err)
		}
		if r == '\n' {
			lineno++
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
	var buf []rune
	readCode := func() string {
		if '{' != r {
			panic(ErrExpectedLBrace)
		}
		buf = []rune{r}
		nesting := 1
		for {
			if read() {
				panic(ErrUnmatchedLBrace)
			}
			buf = append(buf, r)
			if '{' == r {
				nesting++
			} else if '}' == r {
				nesting--
				if 0 == nesting {
					break
				}
			}
		}
		return string(buf)
	}
	var root rule
	needRootRAngle := false
	var parse func(*rule) error
	parse = func(node *rule) error {
		for {
			panicIf(skipws, ErrUnexpectedEOF)
			if '<' == r {
				if node != &root || len(node.kid) > 0 {
					panic(ErrUnexpectedLAngle)
				}
				panicIf(skipws, ErrUnexpectedEOF)
				node.startCode = readCode()
				needRootRAngle = true
				continue
			} else if '>' == r {
				if node == &root {
					if !needRootRAngle {
						panic(ErrUnmatchedRAngle)
					}
				}
				if skipws() {
					return ErrUnexpectedEOF
				}
				node.endCode = readCode()
				return nil
			}
			delim := r
			panicIf(read, ErrUnexpectedEOF)
			var regex []rune
			for {
				if r == delim && (len(regex) == 0 || regex[len(regex)-1] != '\\') {
					break
				}
				if '\n' == r {
					return ErrUnexpectedNewline
				}
				regex = append(regex, r)
				panicIf(read, ErrUnexpectedEOF)
			}
			if "" == string(regex) {
				break
			}
			panicIf(skipws, ErrUnexpectedEOF)
			x := new(rule)
			x.id = fmt.Sprintf("%d", lineno)
			node.kid = append(node.kid, x)
			x.regex = make([]rune, len(regex))
			copy(x.regex, regex)
			if '<' == r {
				panicIf(skipws, ErrUnexpectedEOF)
				x.startCode = readCode()
				parse(x)
			} else {
				x.code = readCode()
			}
		}
		return nil
	}
	err := parse(&root)
	if err != nil {
		return err
	}

	buf = nil
	for done := skipws(); !done; done = read() {
		buf = append(buf, r)
	}
	fs := token.NewFileSet()
	// Append a blank line to make things easier when there are only package and
	// import declarations.
	t, err := parser.ParseFile(fs, "", string(buf)+"\n", parser.ImportsOnly)
	if err != nil {
		panic(err)
	}
	printer.Fprint(out, fs, t)

	var file *token.File
	fs.Iterate(func(f *token.File) bool {
		file = f
		return true
	})

	// Skip over package and import declarations. This is why we appended a blank
	// line above.
	for m := file.LineCount(); m > 1; m-- {
		i := 0
		for '\n' != buf[i] {
			i++
		}
		buf = buf[i+1:]
	}

	prefixReplacer.WriteString(out, lexertext)

	for _, kid := range root.kid {
		gen(out, kid)
	}
	prefixReplacer.WriteString(out, lexeroutro)
	if !standalone {
		writeLex(out, root)
		out.WriteString(string(buf))
		out.Flush()
		if len(outFilename) > 0 {
			gofmt()
		}
		return nil
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
			writeNNFun(out, root)
			buf = buf[m:]
			m = 0
		}
	}
	out.WriteString(string(buf))
	out.Flush()
	if len(outFilename) > 0 {
		gofmt()
	}
	return nil
}

func gofmt() {
	src, err := ioutil.ReadFile(outFilename)
	if err != nil {
		return
	}
	src, err = format.Source(src)
	if err != nil {
		return
	}
	ioutil.WriteFile(outFilename, src, 0666)
}

func panicIf(f func() bool, err error) {
	if f() {
		panic(err)
	}
}

func dieIf(cond bool, v ...interface{}) {
	if cond {
		log.Fatal(v...)
	}
}

func dieErr(err error, s string) {
	if err != nil {
		log.Fatalf("%v: %v", s, err)
	}
}

func createDotFile(filename string) *os.File {
	if filename == "" {
		return nil
	}
	suf := strings.HasSuffix(filename, ".nex")
	dieIf(suf, "nex: DOT filename ends with .nex:", filename)
	file, err := os.Create(filename)
	dieErr(err, "Create")
	return file
}
