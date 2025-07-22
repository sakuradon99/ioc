package ioc

import (
	"encoding/json"
	"errors"
	ioc "github.com/sakuradon99/ioc/internal"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
)

type ValueProvider = ioc.ValueProvider

type MapValueProvider struct {
	valueMap map[string]any
}

func NewMapValueProvider(valueMap map[string]any) *MapValueProvider {
	return &MapValueProvider{valueMap: valueMap}
}

func (m *MapValueProvider) Provide() (map[string]any, error) {
	return m.valueMap, nil
}

type FileValueProvider struct {
	file string
}

func NewFileValueProvider(file string) *FileValueProvider {
	return &FileValueProvider{file: file}
}

func (f *FileValueProvider) Provide() (map[string]any, error) {
	content, err := os.ReadFile(f.file)
	if err != nil {
		return nil, err
	}

	valueMap := make(map[string]any)
	if strings.HasSuffix(f.file, ".json") {
		err = json.Unmarshal(content, &valueMap)
	} else if strings.HasSuffix(f.file, ".yaml") || strings.HasSuffix(f.file, ".yml") {
		err = yaml.Unmarshal(content, &valueMap)
	} else {
		return nil, errors.New("unsupported file format, only .json, .yaml, and .yml are supported")
	}

	if err != nil {
		return nil, err
	}

	return valueMap, nil
}
