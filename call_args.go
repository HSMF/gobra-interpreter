package main

type call_args struct {
	exprs []Expr
}

func (c *call_args) Visit(expr Expr) {
	switch e := expr.(type) {
	case Call:
		for _, v := range e.args {
			_, ok := v.ToValue()
			if !ok {
				c.exprs = append(c.exprs, v)
			}
		}
	}
}
