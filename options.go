package ioc

import ioc "github.com/sakuradon99/ioc/internal"

func Name(nameExpr string) ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.NameExpr = nameExpr
	}
}

func Optional() ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.Optional = true
	}
}

func Constructor(constructor any) ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.Constructor = constructor
	}
}

func Conditional(expr string) ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.ConditionExpr = expr
	}
}
