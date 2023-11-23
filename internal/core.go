package ioc

import (
	"fmt"
	"reflect"
	"strings"
)

type Container interface {
	Register(object any, opts ...RegisterOption) error
	GetObject(name string, target any) (any, error)
}

type ContainerImpl struct {
	objectPool    *ObjectPool
	interfacePool *InterfacePool
	configFetcher *ConfigFetcherImpl
	loaded        bool
}

func NewContainerImpl() *ContainerImpl {
	configFetcher := NewConfigFetcher()
	conditionExecutor := NewConditionExecutorImpl(configFetcher)
	return &ContainerImpl{
		objectPool:    NewObjectPool(conditionExecutor),
		interfacePool: NewInterfacePool(),
		configFetcher: configFetcher,
	}
}

func (c *ContainerImpl) Register(object any, opts ...RegisterOption) error {
	var options RegisterOptions
	for _, opt := range opts {
		opt(&options)
	}

	ot := reflect.TypeOf(object)

	if ot.Kind() != reflect.Ptr {
		return fmt.Errorf("unsupported register type %s", ot.Kind())
	}

	ot = ot.Elem()
	objectID := genObjectID(ot.PkgPath(), ot.Name(), options.Name)

	var obj *Object
	if options.Constructor != nil {
		ct := reflect.TypeOf(options.Constructor)
		if ct.Kind() != reflect.Func {
			return fmt.Errorf("unsupported constructor type %s", ct.Kind())
		}
		if ct.NumOut() > 2 || ct.NumIn() == 0 ||
			ct.NumOut() == 2 && ct.Out(1).Name() != "error" ||
			ct.Out(0).Name() != ot.Name() {
			return fmt.Errorf("unsupported constructor")
		}

		var dependencies []Dependency
		for i := 0; i < ct.NumIn(); i++ {
			dependencies = append(dependencies, Dependency{
				isInterface: ct.In(i).Kind() == reflect.Interface,
				pkgPath:     ct.In(i).PkgPath(),
				name:        ct.In(i).Name(),
			})
		}
		obj = NewObject(
			objectID,
			options.Name,
			options.ConditionExpr,
			dependencies,
			NewConstructorInstanceBuilder(options.Constructor),
		)
	} else {
		var injectFieldIndexes []int
		var dependencies []Dependency
		for i := 0; i < ot.NumField(); i++ {
			field := ot.Field(i)
			if alisa, ok := field.Tag.Lookup("inject"); ok {
				injectFieldIndexes = append(injectFieldIndexes, i)
				ft := field.Type
				var isInterface bool
				if ft.Kind() == reflect.Interface {
					isInterface = true
				} else if ft.Kind() == reflect.Ptr {
					ft = ft.Elem()
				} else {
					return fmt.Errorf(
						"unsupported inject field type <%s>, only pointer and interface type are allowed",
						ft.Kind(),
					)
				}

				dependencies = append(dependencies, Dependency{
					isInterface: isInterface,
					pkgPath:     ft.PkgPath(),
					name:        ft.Name(),
					alisa:       alisa,
				})
			}
		}
		obj = NewObject(
			objectID,
			options.Name,
			options.ConditionExpr,
			dependencies,
			NewFieldInstanceBuilder(ot, injectFieldIndexes),
		)
	}

	err := c.objectPool.Add(obj)
	if err != nil {
		return err
	}

	for _, implementInterface := range options.ImplementInterfaces {
		it := reflect.TypeOf(implementInterface).Elem()
		infID := genInterfaceID(it.PkgPath(), it.Name())
		c.interfacePool.Add(Interface{
			id: infID,
		})
		err = c.interfacePool.BindImplement(infID, objectID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ContainerImpl) GetObject(name string, target any) (any, error) {
	err := c.load()
	if err != nil {
		return nil, err
	}

	tt := reflect.TypeOf(target).Elem()
	id := genObjectID(tt.PkgPath(), tt.Name(), name)

	object, err := c.objectPool.Get(id)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("object %s not found", id)
	}

	return object.instance, nil
}

func (c *ContainerImpl) load() error {
	if c.loaded {
		return nil
	}

	err := c.configFetcher.Load()
	if err != nil {
		return err
	}

	requiredObjects := c.objectPool.List()

	for _, object := range requiredObjects {
		if object.inited {
			continue
		}
		if object.optional {
			continue
		}
		err = c.init(object)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ContainerImpl) init(object *Object) error {
	if object.initializing {
		return fmt.Errorf("circular dependency detected")
	}
	object.initializing = true

	var args []any

	for _, dependency := range object.dependencies {
		var dependencyID string
		var dependencyObject *Object
		if dependency.isInterface {
			interfaceID := genInterfaceID(dependency.pkgPath, dependency.name)
			objectIDs, err := c.interfacePool.GetImplementObjectIDs(interfaceID)
			if err != nil {
				return err
			}

			if len(objectIDs) == 0 {
				return fmt.Errorf("missing implementation for interface %s", interfaceID)
			}

			var findImpl bool
			for _, objectID := range objectIDs {
				if strings.HasSuffix(objectID, "-"+dependency.alisa) {
					dependencyID = objectID
					dependencyObject, err = c.objectPool.Get(dependencyID)
					if err != nil {
						return err
					}
					if dependencyObject != nil {
						if findImpl {
							return fmt.Errorf("ambiguous implementation for interface %s", interfaceID)
						}
						findImpl = true
					}
				}
			}
		} else {
			dependencyID = genObjectID(dependency.pkgPath, dependency.name, dependency.alisa)
			var err error
			dependencyObject, err = c.objectPool.Get(dependencyID)
			if err != nil {
				return err
			}
		}

		if dependencyObject == nil {
			return fmt.Errorf("missing dependency object %s", dependencyID)
		}
		if !dependencyObject.inited {
			err := c.init(dependencyObject)
			if err != nil {
				return err
			}
		}
		args = append(args, dependencyObject.instance)
	}

	instance, err := object.instanceBuilder.Build(args)
	if err != nil {
		return err
	}

	it := reflect.TypeOf(instance).Elem()
	for i := 0; i < it.NumField(); i++ {
		field := it.Field(i)
		if configExpr, ok := field.Tag.Lookup("value"); ok {
			value, exist := c.configFetcher.Fetch(configExpr)
			if !exist {
				return fmt.Errorf("config %s not found", configExpr)
			}
			assignPrivateField(reflect.ValueOf(instance).Elem().Field(i), value)
		}
	}

	object.instance = instance
	object.inited = true
	object.initializing = false
	return nil
}
