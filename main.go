package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type binop int

// this is hacky but i cba to add a parameter to the String function
var indentLevel []int = []int{0}

const (
	add binop = iota
	mul
	concat
	sub
	div
	eqeq
)

func assert(b bool, reason ...string) {
	if !b {
		panic(strings.Join(reason, ", "))
	}
}

func evalBinop(op binop, l, r Val) Val {
	li, lint := l.(Int)
	ri, rint := r.(Int)
	ls, lseq := l.(Seq)
	rs, rseq := r.(Seq)
	switch op {
	case add:
		assert(lint && rint)
		return Int{li.val + ri.val}
	case sub:
		assert(lint && rint)
		return Int{li.val - ri.val}
	case mul:
		assert(lint && rint)
		return Int{li.val * ri.val}
	case div:
		assert(lint && rint)
		return Int{li.val / ri.val}
	case concat:
		assert(lseq && rseq)
		return Seq{"", append(ls.elems, rs.elems...)}
	case eqeq:
		return Bool{l.Equals(r)}
	default:
		panic("unsupported binop")
	}
}

type Ctx struct {
	fns []Func
	callExprs []Call
	criticalExprs []Expr
}

func (c *Ctx) getFn(name string) Func {
	for _, f := range c.fns {
		if f.Name == name {
			return f
		}
	}

	panic(fmt.Sprintf("function %s not found", name))
}

type Expr interface {
	Step(*Ctx) Expr
	ToValue() (Val, bool)
	Subst(string, Expr) Expr
	String() string
}

func exprsString(e []Expr) []string {
	res := make([]string, len(e))
	for i, ex := range e {
		res[i] = ex.String()
	}
	return res
}

type Binop struct {
	opcode binop
	l      Expr
	r      Expr
}

func (b Binop) Step(c *Ctx) Expr {
	l := b.l.Step(c)
	vl, ok := l.ToValue()
	if !ok {
		return Binop{b.opcode, l, b.r}
	}
	r := b.r.Step(c)
	vr, ok := r.ToValue()
	if !ok {
		return Binop{b.opcode, l, r}
	}

	return lit(evalBinop(b.opcode, vl, vr))
}

func (b Binop) String() string {
	op := "?"
	switch b.opcode {
	case add:
		op = "+"
	case mul:
		op = "*"
	case concat:
		op = "++"
	case sub:
		op = "-"
	case div:
		op = "/"
	case eqeq:
		op = "=="
	}
	return fmt.Sprintf("(%s %s %s)", b.l.String(), op, b.r.String())
}

func (b Binop) ToValue() (Val, bool) {
	return nil, false
}

func (b Binop) Subst(s string, to Expr) Expr {
	return Binop{b.opcode, b.l.Subst(s, to), b.r.Subst(s, to)}
}

type Ternop struct {
	cond Expr
	yes  Expr
	no   Expr
}

func (t Ternop) String() string {
	level := indentLevel[len(indentLevel)-1]
	indent := strings.Repeat("\t", level+1)

	indentLevel = append(indentLevel, level+1)
	res := fmt.Sprintf("(%s?\n%s%s:\n%s%s)", t.cond.String(), indent, t.yes.String(), indent, t.no.String())
	indentLevel = indentLevel[:len(indentLevel)-1]
	return res
}

func (t Ternop) Step(c *Ctx) Expr {
	cond := t.cond.Step(c)
	val, ok := cond.ToValue()
	if !ok {
		return Ternop{cond, t.yes, t.no}
	}

	valb, ok := val.(Bool)
	if !ok {
		panic("non-boolean condition")
	}


	if valb.val {
		return t.yes
	} else {
		return t.no
	}
}

func (b Ternop) ToValue() (Val, bool) {
	return nil, false
}

func (b Ternop) Subst(s string, to Expr) Expr {
	return Ternop{b.cond.Subst(s, to), b.yes.Subst(s, to), b.no.Subst(s, to)}
}

type Call struct {
	name string
	args []Expr
}

func call(name string, args ...Expr) Call {
	return Call{name, args}
}

func (t Call) String() string {
	return fmt.Sprintf("%s(%s)", t.name, strings.Join(exprsString(t.args), ", "))
}

func (t Call) Step(c *Ctx) Expr {
	args := append([]Expr{}, t.args...)

	for i, arg := range args {
		args[i] = arg.Step(c)
		_, ok := args[i].ToValue()
		if !ok {
			return Call{t.name, args}
		}
	}

	if t.name == "len" {
		v, _ := args[0].ToValue()
		return IntLit{len(asSeq(v).elems)}
	}

	c.callExprs = append(c.callExprs, Call{t.name, args})
	c.criticalExprs = append(c.criticalExprs, Call{t.name, args})

	fun := c.getFn(t.name)
	res := fun.body

	assert(len(fun.vars) == len(args))
	for i, name := range fun.vars {
		res = res.Subst(name, args[i])
	}

	return res
}

func (b Call) ToValue() (Val, bool) {
	return nil, false
}

func (b Call) Subst(s string, to Expr) Expr {
	args := make([]Expr, len(b.args))
	for i, arg := range b.args {
		args[i] = arg.Subst(s, to)
	}

	return Call{b.name, args}
}

type SeqLit struct {
	typ  string
	args []Expr
}

func (t SeqLit) String() string {
	return fmt.Sprintf("%s{%s}", t.typ, strings.Join(exprsString(t.args), ", "))
}

func (t SeqLit) Step(c *Ctx) Expr {
	elems := append([]Expr{}, t.args...)

	for i, arg := range elems {
		elems[i] = arg.Step(c)
		_, ok := elems[i].ToValue()
		if !ok {
			return SeqLit{t.typ, elems}
		}
	}
	return SeqLit{t.typ, elems}
}

func (b SeqLit) ToValue() (Val, bool) {
	v := make([]Val, len(b.args))
	var ok bool
	for i, el := range b.args {
		v[i], ok = el.ToValue()
		if !ok {
			return nil, false
		}
	}

	return Seq{b.typ, v}, true
}

func (b SeqLit) Subst(s string, to Expr) Expr {
	args := make([]Expr, len(b.args))
	for i, arg := range b.args {
		args[i] = arg.Subst(s, to)
	}

	return SeqLit{b.typ, args}
}

type StructLit struct {
	typ    string
	fields map[string]Expr
}

func (t StructLit) String() string {
	res := strings.Builder{}
	res.WriteString(t.typ)
	res.WriteByte('{')

	for k, v := range t.fields {
		res.WriteString(k)
		res.WriteByte(':')
		res.WriteString(v.String())
		res.WriteByte(',')
	}

	res.WriteByte('}')

	return res.String()
}

func (t StructLit) Step(c *Ctx) Expr {
	elems := make(map[string]Expr)
	for k, v := range t.fields {
		elems[k] = v
	}

	for k, v := range t.fields {
		elems[k] = v.Step(c)
		_, ok := elems[k].ToValue()
		if !ok {
			return StructLit{t.typ, elems}
		}
	}
	return StructLit{t.typ, elems}
}

func (b StructLit) ToValue() (Val, bool) {
	fields := make(map[string]Val)
	var ok bool

	for k, v := range b.fields {
		fields[k], ok = v.ToValue()
		if !ok {
			return nil, false
		}
	}

	return Struct{b.typ, fields}, true
}

func (b StructLit) Subst(s string, to Expr) Expr {
	args := make(map[string]Expr)
	for field, val := range b.fields {
		args[field] = val.Subst(s, to)
	}

	return StructLit{b.typ, args}
}

type SeqSlice struct {
	s    Expr
	low  Expr
	high Expr
}

func (t SeqSlice) String() string {
	low := ""
	high := ""
	if t.low != nil {
		low = t.low.String()
	}
	if t.high != nil {
		high = t.high.String()
	}

	return fmt.Sprintf("%s[%s:%s]", t.s.String(), high, low)
}

func (t SeqSlice) Step(c *Ctx) Expr {
	s := t.s.Step(c)
	s2, ok := s.ToValue()
	if !ok {
		return SeqSlice{s, t.low, t.high}
	}

	seq := asSeq(s2)

	var low int = 0
	var lowRed Expr
	var high int = len(seq.elems)

	if t.low != nil {
		lowRed = t.low.Step(c)
		l, ok := lowRed.ToValue()
		if !ok {
			return SeqSlice{s, lowRed, t.high}
		}
		low = asInt(l)
	}

	if t.high != nil {
		highRed := t.high.Step(c)
		h, ok := highRed.ToValue()
		if !ok {
			return SeqSlice{s, lowRed, highRed}
		}
		high = asInt(h)
	}

	res := make([]Expr, high-low)

	for i, v := range seq.elems[low:high] {
		res[i] = lit(v)
	}

	return SeqLit{seq.typ, res}
}

func (b SeqSlice) ToValue() (Val, bool) {
	return nil, false
}

func (b SeqSlice) Subst(s string, to Expr) Expr {
	seq := b.s.Subst(s, to)
	var low Expr
	var high Expr

	if b.low != nil {
		low = b.low.Subst(s, to)
	}

	if b.high != nil {
		high = b.high.Subst(s, to)
	}

	return SeqSlice{seq, low, high}
}

type SeqIndex struct {
	s Expr
	i Expr
}

func (s SeqIndex) String() string {
	return fmt.Sprintf("%s[%s]", s.s.String(), s.i.String())
}

func (t SeqIndex) Step(c *Ctx) Expr {
	s := t.s.Step(c)
	seq, ok := s.ToValue()
	if !ok {
		return SeqIndex{s, t.i}
	}

	i := t.i.Step(c)
	index, ok := i.ToValue()
	if !ok {
		return SeqIndex{s, i}
	}

	return lit(asSeq(seq).elems[asInt(index)])
}

func (b SeqIndex) ToValue() (Val, bool) {
	return nil, false
}

func (b SeqIndex) Subst(s string, to Expr) Expr {
	return SeqIndex{b.s.Subst(s, to), b.i.Subst(s, to)}
}

type IntLit struct {
	val int
}

func (t IntLit) Step(c *Ctx) Expr {
	return t
}

func (b IntLit) ToValue() (Val, bool) {
	return Int{b.val}, true
}

func (b IntLit) Subst(s string, to Expr) Expr {
	return b
}

func (b IntLit) String() string {
	// fmt.Println("hi")
	if ('a' <= b.val && b.val <= 'z') || ('A' <= b.val && b.val <= 'Z') || b.val == '/' || b.val == ',' {
		return fmt.Sprintf("'%c'", b.val)
	}
	return strconv.Itoa(b.val)
}

type BoolLit struct {
	val bool
}

func (b BoolLit) String() string {
	if b.val {
		return "true"
	}
	return "false"
}

func (t BoolLit) Step(c *Ctx) Expr {
	return t
}

func (b BoolLit) ToValue() (Val, bool) {
	return Bool{b.val}, true
}

func (b BoolLit) Subst(s string, to Expr) Expr {
	return b
}

func lit(v Val) Expr {
	switch val := v.(type) {
	case Seq:
		elems := make([]Expr, len(val.elems))
		for i, e := range val.elems {
			elems[i] = lit(e)
		}
		return seq(elems...)
	case Int:
		return IntLit{val.val}
	case Bool:
		return BoolLit{val.val}
	case Struct:
		elems := make(map[string]Expr)
		for k, v := range val.fields {
			elems[k] = lit(v)
		}
		return StructLit{val.typ, elems}
	}
	panic("")
}

func chr(i byte) IntLit {
	return IntLit{int(i)}
}

func seq(args ...Expr) SeqLit {

	typ := "seq[byte]"

	for _, arg := range args {
		a, ok := arg.(SeqLit)
		if !ok {
			continue
		}

		typ = fmt.Sprintf("seq[%s]", a.typ)
		break
	}

	return SeqLit{typ, args}
}

type Var struct {
	Name string
}

func (v Var) String() string {
	return v.Name
}

func (t Var) Step(c *Ctx) Expr {
	panic("this shouldn't happen?")
	return t
}

func (b Var) ToValue() (Val, bool) {
	return nil, false
}

func (b Var) Subst(s string, to Expr) Expr {
	if s == b.Name {
		return to
	}
	return b
}

func v(s string) Var {
	return Var{s}
}

type Func struct {
	Name string
	body Expr
	vars []string
}

func SpecSplit() Func {
	return Func{
		Name: "bytes.SpecSplit",
		vars: []string{"b", "sep"},
		body: Call{
			name: "bytes.SpecSplitInner",
			args: []Expr{v("b"), v("sep"), seq()},
		},
	}
}

func SpecSplitInner() Func {
	return Func{
		Name: "bytes.SpecSplitInner",
		vars: []string{"s", "sep", "ac"},
		body: Ternop{
			Binop{eqeq, call("len", v("s")), IntLit{0}},
			Ternop{
				Binop{eqeq, call("len", v("ac")), IntLit{0}},
				seq(),
				seq(v("ac")),
			},
			Ternop{
				Binop{eqeq, v("sep"), v("s")},
				seq(v("ac"), seq()),
				Ternop{
					Binop{eqeq, SeqSlice{v("s"), nil, call("len", v("sep"))}, v("sep")},
					Binop{
						concat,
						seq(v("ac")),
						call("bytes.SpecSplitInner",
							SeqSlice{v("s"), call("len", v("sep")), nil},
							v("sep"),
							seq(),
						),
					},
					call("bytes.SpecSplitInner",
						SeqSlice{v("s"), IntLit{1}, nil},
						v("sep"),
						Binop{concat, v("ac"), seq(SeqIndex{v("s"), IntLit{0}})},
					),
				},
			},
		},
	}
}

func ToPath() Func {
	// return toPath( bytes.SpecSplit(pathContents(path), seq[byte]{'/'} ), isRooted(path))
	return Func{
		Name: "ToPath",
		vars: []string{"path"},
		body: call("toPath",
			call("bytes.SpecSplit", call("pathContents", v("path")), seq(chr('/'))),
			call("isRooted", v("path")),
		),
	}
}

func newPath() Func {
	// return Path{fields: seq[Segment]{}, rooted: rooted}
	fields := make(map[string]Expr)
	fields["parts"] = seq()
	fields["rooted"] = v("rooted")
	return Func{
		Name: "newPath",
		vars: []string{"rooted"},
		body: StructLit{typ: "Path", fields: fields},
	}
}

func seqStr(s string) SeqLit {
	res := make([]Expr, 0)
	for _, el := range strings.Split(s, "") {
		codepoint, _ := utf8.DecodeRune([]byte(el))
		res = append(res, IntLit{int(codepoint)})
	}
	return seq(res...)
}

func reduceUntilVal(e Expr, c *Ctx) Val {
	for n := 0; ; n++ {
		// fmt.Println()
		// fmt.Printf("iteration %d: %v\n", n, e)
		e = e.Step(c)

		v, ok := e.ToValue()
		if ok {
			return v
		}
	}
}

func evaluatesTo(e Expr, c Ctx) Val {
	return reduceUntilVal(e, &c)
}

func main() {
	c := Ctx{
		[]Func{
			SpecSplit(),
			SpecSplitInner(),
		},
		[]Call{},
		[]Expr{},
	}

	s := seqStr("a,b")
	sep := seqStr(",")

	fmt.Printf("s: %v\n", s.String())
	fmt.Printf("sep: %v\n", sep.String())
	// fmt.Printf("c: %v\n", c)

	var exp Expr
	exp = call("bytes.SpecSplit", s, sep)
	fmt.Printf("exp: %v\n", exp)
	exp = exp.Step(&c)
	fmt.Printf("exp: %v\n", exp)

	val := reduceUntilVal(exp, &c)
	fmt.Printf("val: %v\n", lit(val))

	// fmt.Println()
	// for _, callExpr := range c.callExprs {
	// 	v := evaluatesTo(callExpr, c)
	// 	fmt.Printf("assert %v == %v\n", callExpr, lit(v))
	// }

	fmt.Println()
	for _, e := range c.criticalExprs {
		v := evaluatesTo(e, c)
		fmt.Printf("assert %v == %v\n", e, lit(v))
	}

	// expr :=
}
