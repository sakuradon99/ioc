package ioc

import (
	"errors"
	"github.com/spf13/cast"
	"reflect"
	"strings"
	"sync"
)

type valueMap map[string]any

func (m valueMap) GetValue(keys []string) any {
	if len(keys) == 0 {
		return nil
	}

	key := keys[0]
	value := m[key]
	if len(keys) == 1 {
		return value
	}

	next, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	return valueMap(next).GetValue(keys[1:])
}

func (m valueMap) SetValue(keys []string, value any) {
	if len(keys) == 0 {
		return
	}

	key := keys[0]
	if len(keys) == 1 {
		m[key] = value
		return
	}

	next, ok := m[key].(valueMap)
	if !ok {
		next = make(valueMap)
		m[key] = next
	}

	next.SetValue(keys[1:], value)
}

type ValueProvider interface {
	Provide() (map[string]any, error)
}

type ValueManager interface {
	// AddValueProvider adds a new value provider to the manager.
	// The last added provider will be used first when retrieving values.
	AddValueProvider(provider ValueProvider)
	GetProperty(expr string) (any, bool, error)
	GetValueWithType(expr string, rtp reflect.Type) (any, bool, error)
}

type valueManagerImpl struct {
	mu             sync.Mutex
	loaded         bool
	valueProviders []ValueProvider
	valueMaps      []valueMap
}

func newValueManagerImpl() *valueManagerImpl {
	return &valueManagerImpl{}
}

func (c *valueManagerImpl) AddValueProvider(provider ValueProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.valueProviders = append(c.valueProviders, provider)
}

func (c *valueManagerImpl) GetProperty(expr string) (any, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.loaded {
		for _, provider := range c.valueProviders {
			if provider == nil {
				continue
			}
			vm, err := provider.Provide()
			if err != nil {
				return nil, false, err
			}
			if vm == nil {
				continue
			}
			c.valueMaps = append(c.valueMaps, vm)
		}

		c.loaded = true
	}

	keys := strings.Split(expr, ".")
	for i := len(c.valueMaps) - 1; i >= 0; i-- {
		vm := c.valueMaps[i]
		if vm == nil {
			continue
		}

		value := vm.GetValue(keys)
		if value != nil {
			return value, true, nil
		}
	}

	return nil, false, nil
}

func (c *valueManagerImpl) GetValueWithType(expr string, rtp reflect.Type) (any, bool, error) {
	property, exist, err := c.GetProperty(expr)
	if err != nil {
		return nil, false, err
	}
	if !exist {
		return nil, false, nil
	}

	v, err := c.convertType(property, rtp)
	if err != nil {
		return nil, false, err
	}

	return v, true, nil
}

func (c *valueManagerImpl) convertType(property any, t reflect.Type) (any, error) {
	var isPtr bool
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
		isPtr = true
	}

	if t.Kind() == reflect.Struct {
		pm, ok := property.(map[string]any)
		if !ok {
			return nil, nil
		}

		entity := reflect.New(t)
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			field := newStructField(entity.Elem().Field(i))

			if f.Anonymous {
				ok, err := c.assignProperty(field, pm)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("failed to assign property")
				}
				continue
			}
			p, ok := f.Tag.Lookup("property")
			if !ok {
				continue
			}
			v, ok := pm[p]
			if !ok {
				continue
			}
			ok, err := c.assignProperty(field, v)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, errors.New("failed to assign property")
			}
		}
		if isPtr {
			return entity.Interface(), nil
		}
		return entity.Elem().Interface(), nil
	} else if t.Kind() == reflect.Slice {
		if isPtr {
			return nil, errors.New("pointer to slice is not supported")
		}

		slice, ok := property.([]any)
		if !ok {
			return nil, nil
		}

		sliceVal := reflect.MakeSlice(t, 0, len(slice))
		for _, a := range slice {
			convertedType, err := c.convertType(a, t.Elem())
			if err != nil {
				return nil, err
			}

			sliceVal = reflect.Append(sliceVal, reflect.ValueOf(convertedType))
		}

		return sliceVal.Interface(), nil
	}

	val, err := c.convertBasicType(property, t, isPtr)
	if err != nil {
		return nil, err
	}

	return val, nil
}

func (c *valueManagerImpl) convertBasicType(property any, t reflect.Type, isPtr bool) (any, error) {
	switch t.Kind() {
	case reflect.String:
		val, err := cast.ToStringE(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Bool:
		val, err := cast.ToBoolE(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Int:
		val, err := cast.ToIntE(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Int8:
		val, err := cast.ToInt8E(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Int16:
		val, err := cast.ToInt16E(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Int32:
		val, err := cast.ToInt32E(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Int64:
		val, err := cast.ToInt64E(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Uint:
		val, err := cast.ToUintE(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Uint8:
		val, err := cast.ToUint8E(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Uint16:
		val, err := cast.ToUint16E(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Uint32:
		val, err := cast.ToUint32E(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Uint64:
		val, err := cast.ToUint64E(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Float32:
		val, err := cast.ToFloat32E(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	case reflect.Float64:
		val, err := cast.ToFloat64E(property)
		if err != nil {
			return nil, err
		}
		if isPtr {
			return &val, nil
		}
		return val, nil
	default:
		return nil, nil
	}
}

func (c *valueManagerImpl) assignProperty(field Field, property any) (bool, error) {
	t := field.Type()

	if t.Kind() == reflect.Slice {
		slice, ok := property.([]any)
		if !ok {
			return false, nil
		}

		for _, a := range slice {
			convertedType, err := c.convertType(a, t.Elem())
			if err != nil {
				return false, err
			}

			field.Append(convertedType)
		}
	} else {
		convertedType, err := c.convertType(property, field.Type())
		if err != nil {
			return false, err
		}
		if convertedType == nil {
			return false, nil
		}
		field.Assign(convertedType)
	}

	return true, nil
}
