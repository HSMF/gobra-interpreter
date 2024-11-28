package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Expr interface {
	Step(*Ctx) (Expr, bool)
	ToValue() (Val, bool)
	Subst(string, Expr) Expr
	String() string
	Type(*Ctx) Type
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

func (b Binop) Step(c *Ctx) (Expr, bool) {
	// fmt.Printf("step binop %v\n", b.opcode)
	l, didStep := b.l.Step(c)
	vl, ok := l.ToValue()
	if !ok || didStep {
		return Binop{b.opcode, l, b.r}, didStep
	}

	if b.opcode == and {
		if !asBool(vl) {
			return BoolLit{false}, true
		}
	}

	r, didStep := b.r.Step(c)
	vr, ok := r.ToValue()
	if !ok || didStep {
		return Binop{b.opcode, l, r}, didStep
	}

	return lit(evalBinop(b.opcode, vl, vr)), true
}

func (b Binop) String() string {
	op := "UNHANDLED"
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
	case gt:
		op = ">"
	case lt:
		op = "<"
	case and:
		op = "&&"
	default:
		panic("unhandled binop" + strconv.Itoa(int(b.opcode)))
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

func (t Ternop) Step(c *Ctx) (Expr, bool) {
	cond, didStep := t.cond.Step(c)
	val, ok := cond.ToValue()
	if !ok || didStep {
		return Ternop{cond, t.yes, t.no}, didStep
	}

	valb, ok := val.(Bool)
	if !ok || didStep {
		panic("non-boolean condition")
	}

	if valb.val {
		return t.yes, true
	} else {
		return t.no, true
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

func (t Call) Step(c *Ctx) (Expr, bool) {
	args := append([]Expr{}, t.args...)
	var didStep bool

	for i, arg := range args {
		var ok bool
		_, ok = args[i].ToValue()
		if !ok || didStep {
			c.criticalExprs = append(c.criticalExprs, args[i])
		}

		args[i], didStep = arg.Step(c)
		_, ok = args[i].ToValue()
		if !ok || didStep {
			return Call{t.name, args}, didStep
		}
	}

	if t.name == "len" {
		v, _ := args[0].ToValue()
		return IntLit{len(asSeq(v).elems)}, true
	}

	c.callExprs = append(c.callExprs, Call{t.name, args})
	c.criticalExprs = append(c.criticalExprs, Call{t.name, args})

	fun := c.getFn(t.name)
	res := fun.body

	assert(len(fun.vars) == len(args), fmt.Sprintf("wrong number of arguments for %s", t.name))
	for i, name := range fun.vars {
		res = res.Subst(name, args[i])
	}
	c.critical = t

	return res, true
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
	typ  Type
	args []Expr
}

func (t SeqLit) String() string {
	typ := t.typ

	return fmt.Sprintf("%s{%s}", typ, strings.Join(exprsString(t.args), ", "))
}

func (t SeqLit) Step(c *Ctx) (Expr, bool) {
	var didStep bool
	anyStep := false
	if t.typ == nil {
		t.typ = t.Type(c)
	}
	elems := append([]Expr{}, t.args...)

	for i, arg := range elems {
		elems[i], didStep = arg.Step(c)
		anyStep = anyStep || didStep
		_, ok := elems[i].ToValue()
		if !ok || didStep {
			return SeqLit{t.Type(c), elems}, didStep
		}
	}
	return SeqLit{t.Type(c), elems}, anyStep
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

func (t StructLit) Step(c *Ctx) (Expr, bool) {
	elems := make(map[string]Expr)
	for k, v := range t.fields {
		elems[k] = v
	}

	var didStep bool
	anyStep := false

	for k, v := range t.fields {
		elems[k], didStep = v.Step(c)
		anyStep = anyStep || didStep
		_, ok := elems[k].ToValue()
		if !ok || didStep {
			return StructLit{t.typ, elems}, anyStep
		}
	}
	return StructLit{t.typ, elems}, anyStep
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

	return fmt.Sprintf("%s[%s:%s]", t.s.String(), low, high)
}

func (t SeqSlice) Step(c *Ctx) (Expr, bool) {
	s, didStep := t.s.Step(c)
	s2, ok := s.ToValue()
	if !ok || didStep {
		return SeqSlice{s, t.low, t.high}, didStep
	}

	seq := asSeq(s2)

	var lowRed Expr
	var highRed Expr

	var low int = 0
	var high int = len(seq.elems)

	if t.low != nil {
		lowRed, didStep = t.low.Step(c)
		l, ok := lowRed.ToValue()
		if !ok || didStep {
			return SeqSlice{s, lowRed, t.high}, didStep
		}
		low = asInt(l)
	}

	if t.high != nil {
		highRed, didStep = t.high.Step(c)
		h, ok := highRed.ToValue()
		if !ok || didStep {
			return SeqSlice{s, lowRed, highRed}, didStep
		}
		high = asInt(h)
	}

	res := make([]Expr, high-low)

	for i, v := range seq.elems[low:high] {
		res[i] = lit(v)
	}

	if low != 0 {
		c.critical = t
	}

	expr := SeqLit{seq.typ, res}
	expr.typ = expr.Type(c)
	return expr, true
}

func (b SeqSlice) ToValue() (Val, bool) {
	return nil, false
}

func (b SeqSlice) Subst(s string, to Expr) Expr {
	// fmt.Printf("%v: %s -> %v\n", reflect.TypeOf(b).Name(), s, to)
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

func (t SeqIndex) Step(c *Ctx) (Expr, bool) {
	s, didStep := t.s.Step(c)
	seq, ok := s.ToValue()
	if !ok || didStep {
		return SeqIndex{s, t.i}, didStep
	}

	i, didStep := t.i.Step(c)
	index, ok := i.ToValue()
	if !ok || didStep {
		return SeqIndex{s, i}, didStep
	}

	return lit(asSeq(seq).elems[asInt(index)]), true
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

func (t IntLit) Step(c *Ctx) (Expr, bool) {
	return t, false
}

func (b IntLit) ToValue() (Val, bool) {
	return Int{b.val}, true
}

func (b IntLit) Subst(s string, to Expr) Expr {
	return b
}

func (b IntLit) String() string {
	if unicode.IsPrint(rune(b.val)) {
		return fmt.Sprintf("'%c'", b.val)
	}
	// if ('a' <= b.val && b.val <= 'z') || ('A' <= b.val && b.val <= 'Z') || b.val == '/' || b.val == ',' {
	// 	return fmt.Sprintf("'%c'", b.val)
	// }
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

func (t BoolLit) Step(c *Ctx) (Expr, bool) {
	return t, false
}

func (b BoolLit) ToValue() (Val, bool) {
	return Bool{b.val}, true
}

func (b BoolLit) Subst(s string, to Expr) Expr {
	return b
}

type SymLit struct {
	val SymVal
}

func (b SymLit) String() string {
	return b.val.e.String()
}

func (t SymLit) Step(c *Ctx) (Expr, bool) {
	return t, false
}

func (b SymLit) ToValue() (Val, bool) {
	return b.val, true
}

func (b SymLit) Subst(s string, to Expr) Expr {
	return b
}

func lit(v Val) Expr {
	switch val := v.(type) {
	case Seq:
		elems := make([]Expr, len(val.elems))
		for i, e := range val.elems {
			elems[i] = lit(e)
		}
		return SeqLit{val.typ, elems}
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
	case SymVal:
		return SymLit{val}
	}
	panic("")
}

func chr(i byte) IntLit {
	return IntLit{int(i)}
}

func tseq(t Type, args ...Expr) SeqLit {
	return SeqLit{TSeq{t}, args}
}

func seq(args ...Expr) SeqLit {
	res := SeqLit{nil, args}
	empty := EmptyCtx()
	res.typ = res.Type(&empty)
	return res
}

type Var struct {
	Name string
}

func (v Var) String() string {
	return v.Name
}

func (t Var) Step(c *Ctx) (Expr, bool) {
	return t, false
}

func (b Var) ToValue() (Val, bool) {
	return SymVal{b}, true
}

func (b Var) Subst(s string, to Expr) Expr {
	if s == b.Name {
		return to
	}
	return b
}

type FieldAccess struct {
	lhs   Expr
	field string
}

func (v FieldAccess) String() string {
	return fmt.Sprintf("%s.%s", v.lhs, v.field)
}

func (t FieldAccess) Step(c *Ctx) (Expr, bool) {
	e, didStep := t.lhs.Step(c)
	lhs, ok := e.ToValue()
	if didStep || !ok {
		return e, didStep
	}
	lhsS, ok := lhs.(Struct)
	if !ok {
		panic(fmt.Sprintf("field access %v requires lhs to be struct", e))
	}

	res, ok := lhsS.fields[t.field]
	if !ok {
		panic(fmt.Sprintf("struct %q does not have field %q", lhsS.typ, t.field))
	}

	return lit(res), true
}

func (b FieldAccess) ToValue() (Val, bool) {
	return nil, false
}

func (b FieldAccess) Subst(s string, to Expr) Expr {
	return FieldAccess{b.lhs.Subst(s, to), b.field}
}

func v(s string) Var {
	return Var{s}
}

type Func struct {
	Name string
	body Expr
	vars []string
	// argtypes []Type
	rettyp Type
}
