package ioc

import ioc "github.com/sakuradon99/ioc/internal"

var iocContainer ioc.Container = ioc.NewContainerImpl()

func Register(object any, opts ...ioc.RegisterOption) any {
	err := iocContainer.Register(object, opts...)
	if err != nil {
		panic(err)
	}

	return nil
}

func GetObject(name string, target any) (any, error) {
	return iocContainer.GetObject(name, target)
}
