package ioc

import ioc "github.com/sakuradon99/ioc/internal"

var iocContainer ioc.Container = ioc.NewContainerImpl()

func Register[T any](opts ...ioc.RegisterOption) any {
	err := iocContainer.Register(new(T), opts...)
	if err != nil {
		panic(err)
	}

	return nil
}

func GetObject[T any](name string) (any, error) {
	return iocContainer.GetObject(name, new(T))
}
