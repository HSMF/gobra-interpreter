package main

import "fmt"

type Val interface {
	Equals(Val) bool
}

type Seq struct {
	typ   Type
	elems []Val
}

func (s Seq) Equals(other Val) bool {
	o, ok := other.(Seq)
	if !ok {
		return false
	}

	if len(o.elems) != len(s.elems) {
		return false
	}

	for i := 0; i < len(s.elems); i++ {
		if !s.elems[i].Equals(o.elems[i]) {
			return false
		}
	}

	return true
}

type Int struct {
	val int
}

func (s Int) Equals(other Val) bool {
	o, ok := other.(Int)
	if !ok {
		return false
	}

	return o.val == s.val
}

type SymVal struct {
	e Expr
}

func (s SymVal) Equals(other Val) bool {
	x, ok := other.(SymVal)
	return ok && (x == s)
}

type Bool struct {
	val bool
}

func (s Bool) Equals(other Val) bool {
	o, ok := other.(Bool)
	if !ok {
		return false
	}

	return o.val == s.val
}

type Struct struct {
	typ    string
	fields map[string]Val
}

func (s Struct) Equals(other Val) bool {
	o, ok := other.(Struct)
	if !ok {
		return false
	}

	if len(o.fields) != len(s.fields) {
		return false
	}
	for k, v := range s.fields {
		if !o.fields[k].Equals(v) {
			return false
		}
	}

	return true
}

func asSeq(v Val) Seq {
	val, ok := v.(Seq)
	if !ok {
		panic(fmt.Sprintf("expected type of %v to be seq but got something else", v))
	}
	return val
}

func asInt(v Val) int {
	val, ok := v.(Int)
	if !ok {
		panic(fmt.Sprintf("expected type of %v to be int but got something else", v))
	}
	return val.val
}

func asBool(v Val) bool {
	val, ok := v.(Bool)
	if !ok {
		panic(fmt.Sprintf("expected type of %v to be bool but got something else", v))
	}
	return val.val
}
