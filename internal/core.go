package ioc

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

type Container interface {
	Register(object any, opts ...RegisterOption) error
	GetObject(name string, target any) (any, error)
}

type ContainerImpl struct {
	objectPool        *ObjectPool
	interfacePool     *InterfacePool
	configFetcher     ConfigFetcher
	conditionExecutor ConditionExecutor
	mu                sync.Mutex
}

func NewContainerImpl() *ContainerImpl {
	configFetcher := NewConfigFetcher()
	conditionExecutor := NewConditionExecutorImpl(configFetcher)
	return &ContainerImpl{
		objectPool:        NewObjectPool(conditionExecutor),
		interfacePool:     NewInterfacePool(),
		configFetcher:     configFetcher,
		conditionExecutor: conditionExecutor,
	}
}

func (c *ContainerImpl) Register(object any, opts ...RegisterOption) error {
	c.mu.Lock()
	defer c.mu.Unlock()

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
		if ct.NumOut() > 2 || ct.NumOut() == 0 ||
			ct.NumOut() == 2 && ct.Out(1).Name() != "error" {
			return fmt.Errorf("unsupported constructor")
		}

		var dependencies []Dependency
		for i := 0; i < ct.NumIn(); i++ {
			param := ct.In(i)
			var isInterface bool
			if param.Kind() == reflect.Interface {
				isInterface = true
			} else if param.Kind() == reflect.Ptr {
				param = param.Elem()
			} else {
				return fmt.Errorf(
					"unsupported constructor param type <%s>, only pointer and interface type are allowed",
					param.Kind(),
				)
			}

			dependencies = append(dependencies, Dependency{
				isInterface: isInterface,
				pkgPath:     param.PkgPath(),
				name:        param.Name(),
			})
		}
		obj = NewObject(
			objectID,
			options.Name,
			options.ConditionExpr,
			options.Optional,
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
			options.Optional,
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
	c.mu.Lock()
	defer c.mu.Unlock()

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
	requiredObjects := c.objectPool.List()

	for _, object := range requiredObjects {
		if object.inited {
			continue
		}
		if object.optional {
			continue
		}
		if object.condition != "" {
			ok, err := c.conditionExecutor.Execute(object.condition)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
		}
		err := c.init(object)
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

			for _, objectID := range objectIDs {
				if strings.HasSuffix(objectID, "-"+dependency.alisa) {
					dependencyID = objectID
					implObj, err := c.objectPool.Get(dependencyID)
					if err != nil {
						return err
					}
					if implObj != nil {
						if dependencyObject != nil {
							return fmt.Errorf("ambiguous implementation for interface %s", interfaceID)
						}
						dependencyObject = implObj
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
		if valueExpr, ok := field.Tag.Lookup("value"); ok {
			arr := strings.Split(valueExpr, ";")
			valueKey := arr[0]
			value, exist, err := c.configFetcher.Fetch(valueKey)
			if err != nil {
				return err
			}
			var optional bool
			if len(arr) > 1 {
				for _, s := range arr {
					if s == "optional" {
						optional = true
					}
				}
			}
			if !optional && !exist {
				return fmt.Errorf("value <%s> not found", valueExpr)
			}
			if exist {
				assignPrivateField(reflect.ValueOf(instance).Elem().Field(i), value)
			}
		}
	}

	object.instance = instance
	object.inited = true
	object.initializing = false
	return nil
}
