package ioc

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

type Container interface {
	Register(ref any, opts ...RegisterOption) error
	GetObject(name string, ref any) (any, error)
}

type ContainerImpl struct {
	objectBuilderFactory ObjectBuilderFactory
	objectPool           ObjectPool
	sourceManager        PropertyManager
	conditionExecutor    ConditionExecutor
	mu                   sync.Mutex
}

func NewContainerImpl() *ContainerImpl {
	sourceManager := newPropertyManagerImpl()
	conditionExecutor := newConditionExecutorImpl(sourceManager)
	objectPool := newObjectPoolImpl(conditionExecutor)
	return &ContainerImpl{
		objectBuilderFactory: newObjectBuilderFactoryImpl(),
		objectPool:           objectPool,
		sourceManager:        sourceManager,
		conditionExecutor:    conditionExecutor,
	}
}

func (c *ContainerImpl) Register(ref any, opts ...RegisterOption) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var options RegisterOptions
	for _, opt := range opts {
		opt(&options)
	}

	ob := c.objectBuilderFactory.GetBuilder(options)
	object, err := ob.Build(ref, options)
	if err != nil {
		return err
	}

	err = c.objectPool.Add(object)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerImpl) GetObject(name string, ref any) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.load()
	if err != nil {
		return nil, err
	}

	of, err := parseObjectRef(ref, name)
	if err != nil {
		return nil, err
	}

	switch of.Type().Kind() {
	case reflect.Interface:
		objects, err := c.objectPool.GetObjectsByInterface(of.Type())
		if err != nil {
			return nil, err
		}
		if len(objects) == 0 {
			return nil, fmt.Errorf("no implementation found for interface %s", of.TypeName())
		}
		var instance any
		for _, object := range objects {
			if object.Name() != name {
				continue
			}
			if instance != nil {
				return nil, fmt.Errorf("ambiguous implementation for interface %s", of.TypeName())
			}
			instance = object.Instance()
		}
		if instance == nil {
			return nil, fmt.Errorf("no implementation found for interface %s", of.TypeName())
		}
		return instance, nil
	case reflect.Struct:
		object, err := c.objectPool.Get(of.ObjectID())
		if err != nil {
			return nil, err
		}
		if object == nil {
			return nil, fmt.Errorf("object %s not found", of.ObjectID())
		}
		return object.Instance(), nil
	default:
		return nil, errors.New("unsupported ref")
	}
}

func (c *ContainerImpl) load() error {
	requiredObjects := c.objectPool.List()

	for _, object := range requiredObjects {
		if object.Status() == ObjectStatusInitialized {
			continue
		}
		if object.Optional() {
			continue
		}
		if object.Condition() != "" {
			ok, err := c.conditionExecutor.Execute(object.Condition())
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
		}
		err := c.initObject(object)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ContainerImpl) initObject(object Object) error {
	if object.Status() == ObjectStatusInitializing {
		return fmt.Errorf("circular dependency detected")
	}
	object.StartInitialization()

	var args []any

	for _, dependency := range object.Dependencies() {
		switch dependency.(type) {
		case *objectDependency:
			dep := dependency.(*objectDependency)
			dependencyObject, err := c.objectPool.Get(dep.ObjectID())
			if err != nil {
				return err
			}
			if dependencyObject == nil {
				if !dependency.Optional() {
					return newMissingObjectError(dep.ObjectID())
				}
				args = append(args, nil)
				continue
			}
			if dependencyObject.Status() != ObjectStatusInitialized {
				err = c.initObject(dependencyObject)
				if err != nil {
					return err
				}
			}
			args = append(args, dependencyObject.Instance())
		case *interfaceDependency:
			dep := dependency.(*interfaceDependency)
			implObjects, err := c.objectPool.GetObjectsByInterface(dep.Type())
			if err != nil {
				return err
			}

			var dependencyObject Object
			for _, implObject := range implObjects {
				if implObject.Name() != dep.Name() {
					continue
				}
				if dependencyObject != nil {
					return fmt.Errorf("ambiguous implementation for interface %s", dep.FullType())
				}
				dependencyObject = implObject
			}

			if dependencyObject == nil {
				if !dependency.Optional() {
					return newMissingImplementationError(dep.FullType())
				}
				args = append(args, nil)
				continue
			}
			if dependencyObject.Status() != ObjectStatusInitialized {
				err = c.initObject(dependencyObject)
				if err != nil {
					return err
				}
			}
			args = append(args, dependencyObject.Instance())
		case *interfaceListDependency:
			dep := dependency.(*interfaceListDependency)
			implObjects, err := c.objectPool.GetObjectsByInterface(dep.Type())
			if err != nil {
				return err
			}

			rInterfaceList := reflect.MakeSlice(reflect.SliceOf(dep.Type()), 0, len(implObjects))
			for _, implObject := range implObjects {
				if implObject.Status() != ObjectStatusInitialized {
					err = c.initObject(implObject)
					if err != nil {
						return err
					}
				}
				rInterfaceList = reflect.Append(rInterfaceList, reflect.ValueOf(implObject.Instance()))
			}
			args = append(args, rInterfaceList.Interface())
		case *valueDependency:
			v, ok, err := c.sourceManager.GetPropertyWithType(dependency.Name(), dependency.Type())
			if err != nil {
				return err
			}
			if !ok && !dependency.Optional() {
				return fmt.Errorf("dependency value <%s> not found", dependency.Name())
			}
			args = append(args, v)
		default:
			return fmt.Errorf("unsupported dependency type %s", reflect.TypeOf(dependency))
		}
	}

	_, err := object.Build(args)
	if err != nil {
		return err
	}

	return nil
}
