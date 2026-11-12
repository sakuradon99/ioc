package ioc

import "strings"

const (
	TagInjectKey   = "inject"
	TagValueKey    = "value"
	TagPropertyKey = "property"

	tagOptionOptional = "optional"
	tagOptionDefault  = "default"
)

// Tag is a parsed struct tag in the form of `value;option1;option2=xxx`.
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

// HasOption reports whether the tag has a flag option, e.g. `optional`.
func (t Tag) HasOption(key string) bool {
	for _, opt := range t.options {
		if opt == key {
			return true
		}
	}
	return false
}

// Option returns the value of a key-value option, e.g. `default=8080`.
func (t Tag) Option(key string) (string, bool) {
	prefix := key + "="
	for _, opt := range t.options {
		if strings.HasPrefix(opt, prefix) {
			return opt[len(prefix):], true
		}
	}
	return "", false
}

type InjectTag struct {
	Tag
}

func ParseInjectTag(tag string) InjectTag {
	return InjectTag{ParseTag(tag)}
}

func (t InjectTag) Optional() bool {
	return t.HasOption(tagOptionOptional)
}

// ValueTag is used by both `value` and `property` tags.
type ValueTag struct {
	Tag
}

func ParseValueTag(tag string) ValueTag {
	return ValueTag{ParseTag(tag)}
}

func (t ValueTag) Optional() bool {
	return t.HasOption(tagOptionOptional)
}

// Default returns the default value used when the value is missing.
func (t ValueTag) Default() (string, bool) {
	return t.Option(tagOptionDefault)
}
