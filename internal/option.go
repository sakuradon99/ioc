package ioc

type RegisterOptions struct {
	NameExpr      string
	Optional      bool
	Constructor   any
	ConditionExpr string
}

type RegisterOption func(o *RegisterOptions)
