package ioc

type RegisterOptions struct {
	Alisa               string
	Optional            bool
	ImplementInterfaces []any
	Constructor         any
	ConditionExpr       string
}

type RegisterOption func(o *RegisterOptions)
