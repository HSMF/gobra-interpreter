package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type binop int

// this is hacky
var indentLevel []int = []int{0}

const (
	add binop = iota
	mul
	concat
	sub
	div
	eqeq
	gt
	lt
	and
)

func assert(b bool, reason ...string) {
	if !b {
		panic(strings.Join(reason, ", "))
	}
}

type Visitor interface {
	Visit(e Expr)
}

func Walk(v Visitor, expr Expr) {
	if expr == nil {
		return
	}
	v.Visit(expr)
	switch e := expr.(type) {
	case Binop:
		Walk(v, e.l)
		Walk(v, e.r)
	case Ternop:
		Walk(v, e.cond)
		Walk(v, e.yes)
		Walk(v, e.no)
	case Call:
		for _, arg := range e.args {
			Walk(v, arg)
		}
	case StructLit:
		for _, ex := range e.fields {
			Walk(v, ex)
		}
	case SeqLit:
		for _, arg := range e.args {
			Walk(v, arg)
		}
	case SeqIndex:
		Walk(v, e.s)
		Walk(v, e.i)
	case SeqSlice:
		Walk(v, e.s)
		Walk(v, e.low)
		Walk(v, e.high)

	}
}

func evalBinop(op binop, l, r Val) Val {

	if _, ok := l.(SymVal); ok {
		return SymVal{Binop{op, lit(l), lit(r)}}
	}
	if _, ok := r.(SymVal); ok {
		return SymVal{Binop{op, lit(l), lit(r)}}
	}

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
		return Seq{ls.typ, append(ls.elems, rs.elems...)}
	case eqeq:
		return Bool{l.Equals(r)}
	case lt:
		return Bool{asInt(l) < asInt(r)}
	case gt:
		return Bool{asInt(l) > asInt(r)}
	case and:
		return Bool{asBool(l) && asBool(r)}
	default:
		panic("unsupported binop")
	}
}

type Ctx struct {
	fns           []Func
	callExprs     []Call
	criticalExprs []Expr
	critical      Expr
}

func EmptyCtx() Ctx {
	return Ctx{
		[]Func{},
		[]Call{},
		[]Expr{},
		nil,
	}
}

func (c Ctx) WithFunctions(f []Func) Ctx {
	c.fns = f
	return c
}

func (c *Ctx) tryGetFn(name string) *Func {
	for _, f := range c.fns {
		if f.Name == name {
			return &f
		}
	}

	return nil
}

func (c *Ctx) getFn(name string) Func {
	res := c.tryGetFn(name)
	if res == nil {
		panic(fmt.Sprintf("function %s not found", name))
	}
	return *res
}

func seqStr(s string) SeqLit {
	res := make([]Expr, 0)
	for _, el := range strings.Split(s, "") {
		codepoint, _ := utf8.DecodeRune([]byte(el))
		res = append(res, IntLit{int(codepoint)})
	}
	return SeqLit{TSeq{tbyte()}, res}
}

func reduceUntilVal(e Expr, c *Ctx) ([]Expr, Val) {
	var didStep bool
	exprs := make([]Expr, 0, 1)
	exprs = append(exprs, e)
	for n := 0; ; n++ {
		// fmt.Println()
		// fmt.Printf("iteration %d: %v\n", n, e)
		e, didStep = e.Step(c)

		v, ok := e.ToValue()
		if ok {
			return exprs, v
		}

		if c.critical != nil {
			exprs = append(exprs, e)
		}
		c.critical = nil

		if !didStep {
			panic("could not make progress but is not value")
		}
	}
}

func generateFunctionCallAssertions(e Expr) {
	w := strings.Builder{}
	fmt.Fprintf(&w, "// reducing %s \n", e)
	c := mkCtx()
	calls, _ := genFnCalls(e, &c)

	for i := len(calls) - 1; i >= 0; i-- {
		_, v := reduceUntilVal(calls[i], &c)
		fmt.Fprintf(&w, "assert %s == %s\n", calls[i], lit(v))
	}

	s := w.String()

	s = rename(s, tseq(tbyte(), IntLit{'/'}), "sep")
	s = rename(s, seq(tseq(tbyte(), IntLit{'.'}, IntLit{'.'})), "tail")
	s = rename(s, seqStr("d"), "sep")
	s = rename(s, seqStr("abcd"), "abcd")
	s = rename(s, seqStr("abc"), "abc")

	fmt.Println(s)
}

func genFnCalls(e Expr, c *Ctx) ([]Expr, Val) {
	var didStep bool
	calls := make([]Expr, 0)
	for {
		e, didStep = e.Step(c)

		v, ok := e.ToValue()
		if ok {
			return calls, v
		}

		if c.critical != nil {
			calls = append(calls, c.critical)
		}
		c.critical = nil

		if !didStep {
			panic("could not make progress but is not value")
		}
	}

}

func evaluatesTo(e Expr, c Ctx) Val {
	_, v := reduceUntilVal(e, &c)
	return v
}

func mkCtx() Ctx {

	return EmptyCtx().WithFunctions([]Func{
		SpecSplit(),
		SpecSplitInner(),
		ToPath(),
		newPath(),
		toPath(),
		pathContents(),
		isRooted(),
		pathAppend(),
		Repeat(),
	})
}

func generateLikelyAssertions(exp Expr) {

	c := mkCtx()

	for _, fn := range c.fns {
		fmt.Printf("%s: %v\n", fn.Name, fn.body)
	}

	// fmt.Printf("exp: %v\n", exp)
	// exp = exp.Step(&c)
	// fmt.Printf("exp: %v\n", exp)

	intermediate, val := reduceUntilVal(exp, &c)
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

	fmt.Println()
	fmt.Println()
	fmt.Println()
	fmt.Println()
	fmt.Println()
	fmt.Println()

	for i := 0; i < len(intermediate)-1; i++ {
		fmt.Printf("assert %v == %v\n", intermediate[i], intermediate[i+1])
	}
}

func rename(src string, e Expr, v string) string {
	return strings.ReplaceAll(src, e.String(), v)
}

func main() {

	s := seqStr("a,b")
	sep := seqStr(",")

	fmt.Printf("s: %v\n", s.String())
	fmt.Printf("sep: %v\n", sep.String())

	fmt.Printf("newPath(): %v\n", newPath().body)
	// fmt.Printf("c: %v\n", c)

	println()
	println()
	println()

	sep = seqStr("/")

	fmt.Println(SpecSplitInner().body)

	// generateLikelyAssertions(call("bytes.SpecSplit", s, sep))
	// generateLikelyAssertions(call("ToPath", tseq(tbyte())))
	// generateLikelyAssertions(call("ToPath", tseq(tbyte(), IntLit{'/'}, IntLit{'a'})))
	// generateLikelyAssertions(call("ToPath", tseq(tbyte(), IntLit{'.'})))
	// generateFunctionCallAssertions(call("ToPath", tseq(tbyte(), IntLit{'.'})))
	// generateFunctionCallAssertions(call("ToPath", tseq(tbyte())))
	// generateFunctionCallAssertions(call("bytes.SpecSplitInner", tseq(tbyte(), IntLit{'.'}, IntLit{'.'}), tseq(tbyte(), IntLit{'/'}), tseq(tbyte())))
	// generateFunctionCallAssertions(call("bytes.SpecSplit", seqStr("abcd"), tseq(tbyte(), IntLit{'a'})))
	generateFunctionCallAssertions(call("bytes.Repeat", seqStr("ab"), IntLit{2}))
	generateFunctionCallAssertions(call("bytes.Repeat", seqStr("ab"), IntLit{0}))
	generateFunctionCallAssertions(call("bytes.Repeat", seqStr("a"), IntLit{4}))
	generateFunctionCallAssertions(call("bytes.SpecSplit", seqStr("abcd"), tseq(tbyte(), IntLit{'d'})))
	generateFunctionCallAssertions(call("bytes.SpecSplit", seqStr(""), tseq(tbyte(), IntLit{'/'})))
	generateFunctionCallAssertions(call("bytes.SpecSplitInner", seqStr("c/"), tseq(tbyte(), IntLit{'/'}), tseq(tbyte(), IntLit{'-'})))
	generateFunctionCallAssertions(call("bytes.SpecSplitInner", seqStr("c/"), tseq(tbyte(), IntLit{'/'}), v("ac")))
	// generateFunctionCallAssertions(call("bytes.SpecSplitInner", seqStr("/"), tseq(tbyte(), IntLit{'/'}), v("ac")))
	// generateFunctionCallAssertions(call("bytes.SpecSplitInner", sep, sep, v("ac")))
	// expr :=
}
