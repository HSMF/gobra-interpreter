package main

func Len(x Expr) Expr {
	return call("len", x)
}

func lastIndex(s Expr) Expr {
	return Binop{sub, Len(s), IntLit{1}}
}

func SpecSplit() Func {
	return Func{
		Name: "bytes.SpecSplit",
		vars: []string{"b", "sep"},
		body: Call{
			name: "bytes.SpecSplitInner",
			args: []Expr{v("b"), v("sep"), tseq(tbyte())},
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
				tseq(TSeq{tbyte()}),
				tseq(TSeq{tbyte()}, v("ac")),
			},
			Ternop{
				Binop{eqeq, v("sep"), v("s")},
				tseq(TSeq{tbyte()}, v("ac"), tseq(tbyte())),
				Ternop{
					Binop{eqeq, SeqSlice{v("s"), nil, call("len", v("sep"))}, v("sep")},
					Binop{
						concat,
						tseq(TSeq{tbyte()}, v("ac")),
						call("bytes.SpecSplitInner",
							SeqSlice{v("s"), call("len", v("sep")), nil},
							v("sep"),
							tseq(tbyte()),
						),
					},
					call("bytes.SpecSplitInner",
						SeqSlice{v("s"), IntLit{1}, nil},
						v("sep"),
						Binop{concat, v("ac"), tseq(tbyte(), SeqIndex{v("s"), IntLit{0}})},
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
			call("bytes.SpecSplit", call("pathContents", v("path")), tseq(tbyte(), chr('/'))),
			call("isRooted", v("path")),
		),
	}
}

func newPath() Func {
	// return Path{fields: seq[Segment]{}, rooted: rooted}
	fields := make(map[string]Expr)
	fields["parts"] = tseq(TAbstract{"Segment"})
	fields["rooted"] = v("rooted")
	return Func{
		Name: "newPath",
		vars: []string{"rooted"},
		body: StructLit{typ: "Path", fields: fields},
	}
}

func toPath() Func {
	// return len(flat) == 0 ?
	//	newPath(rooted) :
	//	pathAppend( toPath(flat[:len(flat)-1], rooted), flat[len(flat)-1] )

	return Func{
		Name: "toPath",
		vars: []string{"flat", "rooted"},
		body: Ternop{
			Binop{eqeq, Len(v("flat")), IntLit{0}},
			call("newPath", v("rooted")),
			call("pathAppend",
				call("toPath",
					SeqSlice{v("flat"), nil, lastIndex(v("flat"))},
					v("rooted"),
				),
				SeqIndex{v("flat"), Binop{sub, Len(v("flat")), IntLit{1}}},
			),
		},
	}
}

func pathContents() Func {

	// return isRooted(p) ?
	// 	p[1:] :
	// 	p

	return Func{
		Name: "pathContents",
		vars: []string{"p"},
		body: Ternop{
			call("isRooted", v("p")),
			SeqSlice{v("p"), IntLit{1}, nil},
			v("p"),
		},
	}
}

func isRooted() Func {
	// return len(p) > 0 && p[0] == '/'

	return Func{
		Name: "isRooted",
		vars: []string{"p"},
		body: Binop{and,
			Binop{gt, Len(v("p")), IntLit{0}},
			Binop{eqeq, SeqIndex{v("p"), IntLit{0}}, IntLit{'/'}},
		},
	}

}

func pathAppend() Func {
	fields := make(map[string]Expr)
	fields["parts"] = Binop{concat, FieldAccess{v("p"), "parts"}, tseq(TAbstract{"Segment"}, v("s"))}
	fields["rooted"] = FieldAccess{v("p"), "rooted"}

	return Func{
		Name:   "pathAppend",
		vars:   []string{"p", "s"},
		rettyp: TAbstract{"Path"},
		body: StructLit{
			"Path",
			fields,
		},
	}
}
