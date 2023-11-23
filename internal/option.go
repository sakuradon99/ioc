package ioc

type RegisterOptions struct {
	Name                string
	Optional            bool
	ImplementInterfaces []any
	Constructor         any
	ConditionExpr       string
}

type RegisterOption func(o *RegisterOptions)
