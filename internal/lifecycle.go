package ioc

type ObjectInitable interface {
	Init() error
}

func processObjectInitializing(instance any) error {
	if instance == nil {
		return nil
	}
	if obj, ok := instance.(ObjectInitable); ok {
		return obj.Init()
	}
	return nil
}
