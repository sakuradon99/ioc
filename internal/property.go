package ioc

import (
	"errors"
	"github.com/spf13/cast"
	"gopkg.in/yaml.v3"
	"os"
	"reflect"
	"strings"
)

var SourceFile = "./config/application.yaml"

type propertyMap map[string]any

func (m propertyMap) GetProperty(keys []string) any {
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

	return propertyMap(next).GetProperty(keys[1:])
}

type PropertyManager interface {
	GetProperty(expr string) (any, bool, error)
	AssignProperty(expr string, field reflect.Value) (bool, error)
}

type SourceManagerImpl struct {
	loaded      bool
	propertyMap propertyMap
}

func NewSourceManagerImpl() *SourceManagerImpl {
	return &SourceManagerImpl{
		loaded:      false,
		propertyMap: make(propertyMap),
	}
}

func (c *SourceManagerImpl) GetProperty(expr string) (any, bool, error) {
	if !c.loaded {
		content, err := os.ReadFile(SourceFile)
		if err != nil {
			return nil, false, err
		}

		err = yaml.Unmarshal(content, &c.propertyMap)
		if err != nil {
			return nil, false, err
		}
	}

	if len(c.propertyMap) == 0 {
		return nil, false, nil
	}

	property := c.propertyMap.GetProperty(strings.Split(expr, "."))
	if property == nil {
		return nil, false, nil
	}

	return property, true, nil
}

func (c *SourceManagerImpl) AssignProperty(expr string, field reflect.Value) (bool, error) {
	property, exist, err := c.GetProperty(expr)
	if err != nil {
		return false, err
	}
	if !exist {
		return false, nil
	}

	return c.assignProperty(NewField(field), property)
}

func (c *SourceManagerImpl) assignProperty(field *Field, property any) (bool, error) {
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

func (c *SourceManagerImpl) convertType(property any, t reflect.Type) (any, error) {
	var isPtr bool
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
		isPtr = true
	}

	if t.Kind() == reflect.Struct {
		pm, ok := property.(propertyMap)
		if !ok {
			return nil, nil
		}

		entity := reflect.New(t)
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			field := NewField(entity.Elem().Field(i))

			if f.Anonymous {
				ok, err := c.assignProperty(field, pm)
				if err != nil {
					return nil, err
				}
				if !ok {
					return errors.New("failed to assign property"), nil
				}
				continue
			}
			p, ok := f.Tag.Lookup("property")
			if !ok {
				continue
			}
			v, ok := pm[p]
			if !ok {
				return nil, nil
			}
			ok, err := c.assignProperty(field, v)
			if err != nil {
				return nil, err
			}
			if !ok {
				return errors.New("failed to assign property"), nil
			}
		}
		if isPtr {
			return entity.Interface(), nil
		}
		return entity.Elem().Interface(), nil
	}

	val, err := c.convertBasicType(property, t, isPtr)
	if err != nil {
		return nil, err
	}

	if isPtr {
		return val, nil
	}

	return val, nil
}

func (c *SourceManagerImpl) convertBasicType(property any, t reflect.Type, isPtr bool) (any, error) {
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
