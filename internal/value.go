package ioc

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cast"
)

const TagPropertyKey = "property"

var durationType = reflect.TypeOf(time.Duration(0))

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

	switch next := value.(type) {
	case map[string]any:
		return valueMap(next).GetValue(keys[1:])
	case valueMap:
		return next.GetValue(keys[1:])
	default:
		return nil
	}
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
	valueProviders []ValueProvider
	// number of providers in valueProviders that have been loaded into valueMaps.
	// Providers added later are loaded lazily on the next read.
	loadedProviders int
	valueMaps       []valueMap
}

func newValueManagerImpl() *valueManagerImpl {
	return &valueManagerImpl{}
}

func (c *valueManagerImpl) AddValueProvider(provider ValueProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.valueProviders = append(c.valueProviders, provider)
}

func (c *valueManagerImpl) loadProviders() error {
	for c.loadedProviders < len(c.valueProviders) {
		provider := c.valueProviders[c.loadedProviders]
		if provider != nil {
			vm, err := provider.Provide()
			if err != nil {
				// not marked as loaded, so it will be retried on the next read
				return err
			}
			if vm != nil {
				c.valueMaps = append(c.valueMaps, vm)
			}
		}
		c.loadedProviders++
	}
	return nil
}

func (c *valueManagerImpl) GetProperty(expr string) (any, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.loadProviders(); err != nil {
		return nil, false, err
	}

	keys := strings.Split(expr, ".")
	for i := len(c.valueMaps) - 1; i >= 0; i-- {
		value := c.valueMaps[i].GetValue(keys)
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

	v, err := convertType(property, rtp)
	if err != nil {
		return nil, false, fmt.Errorf("value %q: %w", expr, err)
	}

	return v, true, nil
}

// convertType converts property to a value whose dynamic type is exactly t.
// It returns an error instead of a nil value when the conversion is not possible,
// except for nil interface values.
func convertType(property any, t reflect.Type) (any, error) {
	switch t.Kind() {
	case reflect.Pointer:
		v, err := convertType(property, t.Elem())
		if err != nil {
			return nil, err
		}
		ptr := reflect.New(t.Elem())
		ptr.Elem().Set(toReflectValue(v, t.Elem()))
		return ptr.Interface(), nil
	case reflect.Interface:
		if property == nil {
			return nil, nil
		}
		if !reflect.TypeOf(property).Implements(t) {
			return nil, newValueConvertError(property, t, nil)
		}
		return property, nil
	case reflect.Struct:
		if t == durationType {
			break
		}
		return convertStruct(property, t)
	case reflect.Slice, reflect.Array:
		return convertSlice(property, t)
	case reflect.Map:
		return convertMap(property, t)
	}

	return convertBasicType(property, t)
}

func convertStruct(property any, t reflect.Type) (any, error) {
	pm, err := toStringMap(property)
	if err != nil {
		return nil, newValueConvertError(property, t, err)
	}

	entity := reflect.New(t).Elem()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		field := newStructField(entity.Field(i))

		tagExpr, hasTag := f.Tag.Lookup(TagPropertyKey)
		if !hasTag {
			// an untagged embedded struct reads its properties from the same level
			if f.Anonymous && (f.Type.Kind() == reflect.Struct ||
				(f.Type.Kind() == reflect.Pointer && f.Type.Elem().Kind() == reflect.Struct)) {
				if err = assignProperty(field, pm); err != nil {
					return nil, fmt.Errorf("embedded field %s: %w", f.Name, err)
				}
			}
			continue
		}

		key := ParseTag(tagExpr).Value()
		v, ok := pm[key]
		if !ok || v == nil {
			continue
		}
		if err = assignProperty(field, v); err != nil {
			return nil, fmt.Errorf("property %q: %w", key, err)
		}
	}
	return entity.Interface(), nil
}

func convertSlice(property any, t reflect.Type) (any, error) {
	rv := reflect.ValueOf(property)
	if property == nil || (rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array) {
		return nil, newValueConvertError(property, t, nil)
	}

	var out reflect.Value
	if t.Kind() == reflect.Array {
		if rv.Len() > t.Len() {
			return nil, newValueConvertError(property, t, fmt.Errorf("too many elements: %d", rv.Len()))
		}
		out = reflect.New(t).Elem()
	} else {
		out = reflect.MakeSlice(t, rv.Len(), rv.Len())
	}
	for i := 0; i < rv.Len(); i++ {
		ev, err := convertType(rv.Index(i).Interface(), t.Elem())
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
		out.Index(i).Set(toReflectValue(ev, t.Elem()))
	}
	return out.Interface(), nil
}

func convertMap(property any, t reflect.Type) (any, error) {
	if t.Key().Kind() != reflect.String {
		return nil, newValueConvertError(property, t, fmt.Errorf("map key must be string"))
	}
	rv := reflect.ValueOf(property)
	if property == nil || rv.Kind() != reflect.Map {
		return nil, newValueConvertError(property, t, nil)
	}

	out := reflect.MakeMapWithSize(t, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		key, err := cast.ToStringE(iter.Key().Interface())
		if err != nil {
			return nil, newValueConvertError(property, t, err)
		}
		ev, err := convertType(iter.Value().Interface(), t.Elem())
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", key, err)
		}
		out.SetMapIndex(reflect.ValueOf(key).Convert(t.Key()), toReflectValue(ev, t.Elem()))
	}
	return out.Interface(), nil
}

func convertBasicType(property any, t reflect.Type) (any, error) {
	if t == durationType {
		d, err := cast.ToDurationE(property)
		if err != nil {
			return nil, newValueConvertError(property, t, err)
		}
		return d, nil
	}

	var (
		v   any
		err error
	)
	switch t.Kind() {
	case reflect.String:
		v, err = cast.ToStringE(property)
	case reflect.Bool:
		v, err = cast.ToBoolE(property)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if err = checkIntegral(property); err == nil {
			var i int64
			if i, err = cast.ToInt64E(property); err == nil {
				if reflect.Zero(t).OverflowInt(i) {
					err = fmt.Errorf("value %d overflows", i)
				}
				v = i
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if err = checkIntegral(property); err == nil {
			var u uint64
			if u, err = cast.ToUint64E(property); err == nil {
				if reflect.Zero(t).OverflowUint(u) {
					err = fmt.Errorf("value %d overflows", u)
				}
				v = u
			}
		}
	case reflect.Float32, reflect.Float64:
		var f float64
		if f, err = cast.ToFloat64E(property); err == nil {
			if reflect.Zero(t).OverflowFloat(f) {
				err = fmt.Errorf("value %v overflows", f)
			}
			v = f
		}
	default:
		return nil, newValueConvertError(property, t, fmt.Errorf("unsupported value type"))
	}
	if err != nil {
		return nil, newValueConvertError(property, t, err)
	}

	// convert to the exact target type, e.g. `type Level int`
	return reflect.ValueOf(v).Convert(t).Interface(), nil
}

// checkIntegral rejects floats with a fractional part (e.g. JSON numbers) instead of silently truncating them.
func checkIntegral(property any) error {
	var f float64
	switch p := property.(type) {
	case float32:
		f = float64(p)
	case float64:
		f = p
	default:
		return nil
	}
	if f != math.Trunc(f) {
		return fmt.Errorf("value %v is not an integer", f)
	}
	return nil
}

func toStringMap(property any) (map[string]any, error) {
	switch m := property.(type) {
	case map[string]any:
		return m, nil
	case valueMap:
		return m, nil
	}
	rv := reflect.ValueOf(property)
	if property == nil || rv.Kind() != reflect.Map {
		return nil, fmt.Errorf("expect a map")
	}
	out := make(map[string]any, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		key, err := cast.ToStringE(iter.Key().Interface())
		if err != nil {
			return nil, err
		}
		out[key] = iter.Value().Interface()
	}
	return out, nil
}

func toReflectValue(v any, t reflect.Type) reflect.Value {
	if v == nil {
		return reflect.Zero(t)
	}
	return reflect.ValueOf(v)
}

func assignProperty(field Field, property any) error {
	v, err := convertType(property, field.Type())
	if err != nil {
		return err
	}
	field.Assign(v)
	return nil
}
