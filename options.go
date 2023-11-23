package ioc

import ioc "github.com/sakuradon99/ioc/internal"

func Name(name string) ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.Name = name
	}
}

func Implement[I any]() ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.ImplementInterfaces = append(o.ImplementInterfaces, new(I))
	}
}

func Optional() ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.Optional = true
	}
}

func Constructor(constructor any) ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.Constructor = true
	}
}

func Conditional(expr string) ioc.RegisterOption {
	return func(o *ioc.RegisterOptions) {
		o.ConditionExpr = expr
	}
}
