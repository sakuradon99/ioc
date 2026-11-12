package ioc

type RegisterOptions struct {
	Name          string
	Aliases       []string
	Optional      bool
	Primary       bool
	Constructor   any
	ConditionExpr string
}

type RegisterOption func(o *RegisterOptions)
