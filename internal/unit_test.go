package ioc

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		expr         string
		wantValue    string
		wantOptional bool
		wantDefault  string
		hasDefault   bool
	}{
		{expr: "", wantValue: ""},
		{expr: "name", wantValue: "name"},
		{expr: ";optional", wantValue: "", wantOptional: true},
		{expr: "name*;optional", wantValue: "name*", wantOptional: true},
		{expr: "name;other", wantValue: "name"},
		{expr: "port;default=8080", wantValue: "port", wantDefault: "8080", hasDefault: true},
		{expr: "s;default=", wantValue: "s", wantDefault: "", hasDefault: true},
		{expr: "s;default=a=b", wantValue: "s", wantDefault: "a=b", hasDefault: true},
		{expr: "s;optional;default=x", wantValue: "s", wantOptional: true, wantDefault: "x", hasDefault: true},
		{expr: "s;defaults=x", wantValue: "s"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			injectTag := ParseInjectTag(tt.expr)
			assert.Equal(t, tt.wantValue, injectTag.Value())
			assert.Equal(t, tt.wantOptional, injectTag.Optional())

			valueTag := ParseValueTag(tt.expr)
			assert.Equal(t, tt.wantValue, valueTag.Value())
			assert.Equal(t, tt.wantOptional, valueTag.Optional())
			defaultValue, ok := valueTag.Default()
			assert.Equal(t, tt.hasDefault, ok)
			assert.Equal(t, tt.wantDefault, defaultValue)
		})
	}
}

func TestValidateDefaultValue(t *testing.T) {
	type named int
	tests := []struct {
		rtp     reflect.Type
		value   string
		wantErr bool
	}{
		{rtp: typeOf[string](), value: "x"},
		{rtp: typeOf[*int](), value: "3"},
		{rtp: typeOf[named](), value: "3"},
		{rtp: typeOf[bool](), value: "true"},
		{rtp: typeOf[float64](), value: "0.5"},
		{rtp: typeOf[int](), value: "x", wantErr: true},
		{rtp: typeOf[uint8](), value: "300", wantErr: true},
		{rtp: typeOf[[]string](), value: "a", wantErr: true},
		{rtp: typeOf[map[string]int](), value: "a", wantErr: true},
		{rtp: typeOf[struct{}](), value: "a", wantErr: true},
	}
	for _, tt := range tests {
		err := validateDefaultValue(tt.rtp, tt.value)
		if tt.wantErr {
			assert.Error(t, err, "%s <- %q", tt.rtp, tt.value)
		} else {
			assert.NoError(t, err, "%s <- %q", tt.rtp, tt.value)
		}
	}
}

func TestValueMapGetValue(t *testing.T) {
	m := valueMap{
		"a": map[string]any{
			"b": valueMap{"c": 1},
		},
		"scalar": "x",
	}

	assert.Equal(t, 1, m.GetValue([]string{"a", "b", "c"}))
	assert.Equal(t, "x", m.GetValue([]string{"scalar"}))
	assert.Nil(t, m.GetValue([]string{"scalar", "child"}), "cannot descend into a scalar")
	assert.Nil(t, m.GetValue([]string{"missing"}))
	assert.Nil(t, m.GetValue(nil))
}

type errProvider struct{ calls int }

func (p *errProvider) Provide() (map[string]any, error) {
	p.calls++
	if p.calls == 1 {
		return nil, assert.AnError
	}
	return map[string]any{"k": "v"}, nil
}

func TestValueManagerRetriesFailedProvider(t *testing.T) {
	m := newValueManagerImpl()
	m.AddValueProvider(mapProvider{"base": "b"})
	p := &errProvider{}
	m.AddValueProvider(p)
	m.AddValueProvider(nil)

	_, _, err := m.GetProperty("k")
	assert.ErrorIs(t, err, assert.AnError)

	v, ok, err := m.GetProperty("k")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "v", v)
	assert.Len(t, m.valueMaps, 2, "earlier providers must not be loaded twice")
}

func TestConditionExecutor(t *testing.T) {
	vm := newValueManagerImpl()
	vm.AddValueProvider(mapProvider{
		"app": map[string]any{"name": "demo", "replicas": 3},
	})
	exec := newConditionExecutorImpl(vm)

	tests := []struct {
		condition string
		want      bool
		wantErr   bool
	}{
		{condition: "", want: true},
		{condition: "#app.name == 'demo'", want: true},
		{condition: "#app.name == 'other'", want: false},
		{condition: "#app.replicas > 2 && #app.name == 'demo'", want: true},
		{condition: "#app.missing == nil", want: true},
		{condition: "1 + 1", wantErr: true},
		{condition: "((", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			got, err := exec.Execute(tt.condition)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
