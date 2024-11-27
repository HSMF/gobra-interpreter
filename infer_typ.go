package main

// func inferTyp(expr Expr) string {
// 	if expr == nil {
// 		return ""
// 	}
// 	switch e := expr.(type) {
// 	case Binop:
// 		return inferTypSeq(e.l, e.r)
// 	case Ternop:
// 		return inferTypSeq(e.yes, e.no)
// 	case Call:
// 		return ""
// 	case StructLit:
// 		return e.typ
// 	case SeqLit:
// 		if e.typ != "" {
// 			return e.typ
// 		}
// 		return inferTypSeq(e.args...)
// 	case SeqIndex:
// 		return inferTypSeq(e.s, e.i)
// 	case SeqSlice:
// 		return inferTypSeq(e.s, e.low, e.high)
//
// 	case IntLit:
// 		// we only deal with sequences of bytes
// 		return "byte"
// 	case BoolLit:
// 		return "bool"
// 	}
// 	return ""
// }

// func inferTypSeq(e ...Expr) string {
// 	for _, e := range e {
// 		t := inferTyp(e)
// 		if t != "" {
// 			return t
// 		}
// 	}
// 	return ""
// }
