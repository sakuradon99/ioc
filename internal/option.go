package ioc

type RegisterOptions struct {
	Name          string
	Aliases       []string
	Optional      bool
	Constructor   any
	ConditionExpr string
}

type RegisterOption func(o *RegisterOptions)
