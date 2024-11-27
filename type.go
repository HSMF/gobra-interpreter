package main

import "fmt"

type Type interface {
	String() string
}

type primitiveKind int

const (
	intKind primitiveKind = 1 + iota
	boolKind
	byteKind
)

type TAbstract struct {
	name string
}

func (t TAbstract) String() string {
	return t.name
}

type TPrim struct {
	kind primitiveKind
}

func tint() TPrim {
	return TPrim{intKind}
}

func tbool() TPrim {
	return TPrim{boolKind}
}

func tbyte() TPrim {
	return TPrim{byteKind}
}

func (t TPrim) String() string {
	switch t.kind {
	case intKind:
		return "int"
	case boolKind:
		return "bool"
	case byteKind:
		return "byte"
	}
	panic("invalid primitiveKind")
}

type TSeq struct {
	elem Type
}

func (t TSeq) String() string {
	return fmt.Sprintf("seq[%s]", t.elem.String())
}

func isAbstract(t Type) bool {
	_, ok := t.(TAbstract)
	return ok
}

func expectSeq(t Type) TSeq {
	typ, ok := t.(TSeq)
	if !ok {
		panic(fmt.Sprintf("expected a sequence type but got %v", t))
	}
	return typ
}

var binopMatch = []primitiveKind{
	eqeq: boolKind,
	add:  intKind,
	mul:  intKind,
	sub:  intKind,
	div:  intKind,
	gt:   boolKind,
	lt:   boolKind,
	and:  boolKind,
}

func (t Binop) Type(c *Ctx) Type {
	if t.opcode == concat {
		return t.l.Type(c)
	}

	return TPrim{binopMatch[t.opcode]}
}

func (t SeqIndex) Type(c *Ctx) Type {
	fmt.Printf("t.s: %#v\n", t.s)
	if isAbstract(t.s.Type(c)) {
		return nil
	}
	return expectSeq(t.s.Type(c)).elem
}
func (t SeqSlice) Type(c *Ctx) Type  { return t.s.Type(c) }
func (t BoolLit) Type(c *Ctx) Type   { return tbool() }
func (t IntLit) Type(c *Ctx) Type    { return tint() }
func (t Ternop) Type(c *Ctx) Type    { return t.yes.Type(c) }
func (t Var) Type(c *Ctx) Type       { return TAbstract{t.Name} }
func (t StructLit) Type(c *Ctx) Type { return TAbstract{t.typ} }
func (t Call) Type(c *Ctx) Type {
	fn := c.tryGetFn(t.name)
	if fn == nil {
		return nil
	}

	return fn.rettyp

}
func (t SeqLit) Type(c *Ctx) Type {
	if t.typ != nil {
		return t.typ
	}
	for _, el := range t.args {
		ty := el.Type(c)
		if ty != nil {
			return TSeq{ty}
		}
	}
	return nil
}
func (t FieldAccess) Type(c *Ctx) Type {
	return TAbstract{"unknown"}
}
