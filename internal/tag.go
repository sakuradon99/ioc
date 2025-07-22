package ioc

import "strings"

const (
	TagInjectKey = "inject"
	TagValueKey  = "value"
)

type Tag struct {
	value   string
	options []string
}

func ParseTag(tag string) Tag {
	var t Tag
	if tag == "" {
		return t
	}

	arr := strings.Split(tag, ";")
	t.value = arr[0]
	if len(arr) > 1 {
		t.options = arr[1:]
	}
	return t
}

func (t Tag) Value() string {
	return t.value
}

func (t Tag) HasOption(key string) bool {
	for _, opt := range t.options {
		if opt == key {
			return true
		}
	}
	return false
}

type InjectTag struct {
	Tag
}

func ParseInjectTag(tag string) InjectTag {
	return InjectTag{ParseTag(tag)}
}

func (t InjectTag) Optional() bool {
	return t.HasOption("optional")
}

type ValueTag struct {
	Tag
}

func ParseValueTag(tag string) ValueTag {
	return ValueTag{ParseTag(tag)}
}

func (t ValueTag) Optional() bool {
	return t.HasOption("optional")
}
