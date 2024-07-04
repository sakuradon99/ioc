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
		err := c.init(object)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ContainerImpl) init(object Object) error {
	if object.Status() == ObjectStatusInitializing {
		return fmt.Errorf("circular dependency detected")
	}
	object.StartInitialization()

	var args []any

	for _, dependency := range object.Dependencies() {
		var dependencyObject Object
		if dependency.IsInterface() {
			implObjects, err := c.objectPool.GetObjectsByInterface(dependency.Type())
			if err != nil {
				return err
			}

			for _, implObject := range implObjects {
				if implObject.Name() != dependency.Name() {
					continue
				}

				if dependencyObject != nil {
					return fmt.Errorf("ambiguous implementation for interface %s", dependency.ID())
				}
				dependencyObject = implObject
			}
		} else {
			var err error
			dependencyObject, err = c.objectPool.Get(dependency.ID())
			if err != nil {
				return err
			}
		}

		if dependencyObject == nil {
			if dependency.Optional() {
				args = append(args, nil)
				continue
			}
			return fmt.Errorf("missing dependency object <%s>", dependency.ID())
		}
		if dependencyObject.Status() != ObjectStatusInitialized {
			err := c.init(dependencyObject)
			if err != nil {
				return err
			}
		}
		args = append(args, dependencyObject.Instance())
	}

	instance, err := object.Build(args)
	if err != nil {
		return err
	}

	err = c.processBeforePropertyAssignation(instance)
	if err != nil {
		return err
	}

	err = c.processPropertyAssignation(instance)
	if err != nil {
		return err
	}

	err = c.processAfterPropertyAssignation(instance)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerImpl) processBeforePropertyAssignation(instance any) error {
	if reflect.TypeOf(instance).Implements(reflect.TypeOf((*ObjectBeforePropertyAssignation)(nil)).Elem()) {
		err := instance.(ObjectBeforePropertyAssignation).BeforePropertyAssignation()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ContainerImpl) processPropertyAssignation(instance any) error {
	it := reflect.TypeOf(instance).Elem()
	for i := 0; i < it.NumField(); i++ {
		field := it.Field(i)
		if valueExpr, ok := field.Tag.Lookup("value"); ok {
			tag := ParseValueTag(valueExpr)
			exist, err := c.sourceManager.AssignProperty(tag.Value(), reflect.ValueOf(instance).Elem().Field(i))
			if err != nil {
				return err
			}
			if !tag.Optional() && !exist {
				return fmt.Errorf("value <%s> not found", valueExpr)
			}
		}
	}
	return nil
}

func (c *ContainerImpl) processAfterPropertyAssignation(instance any) error {
	if reflect.TypeOf(instance).Implements(reflect.TypeOf((*ObjectAfterPropertyAssignation)(nil)).Elem()) {
		err := instance.(ObjectAfterPropertyAssignation).AfterPropertyAssignation()
		if err != nil {
			return err
		}
	}
	return nil
}
