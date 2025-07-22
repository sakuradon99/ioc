package ioc

type ObjectInitializing interface {
	Init() error
}

func processObjectInitializing(instance any) error {
	if oi, ok := instance.(ObjectInitializing); ok {
		return oi.Init()
	}
	return nil
}
