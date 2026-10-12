package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ValueState identifies where a resolved value came from.
type ValueState uint8

const (
	// ValueOmitted means no token or declared default supplied the value.
	ValueOmitted ValueState = iota
	// ValueDefaulted means the declared default supplied the value.
	ValueDefaulted
	// ValueExplicit means argv supplied the value, including an empty value.
	ValueExplicit
)

type resolvedValue struct {
	value any
	state ValueState
}

// Input is an invocation-local binding lookup. Its zero value yields omitted
// states and zero values. Get copies exact []string and map[string]string values,
// but custom pointers, slices, maps and structs are returned by ordinary Go value
// assignment, not deep cloning; callers own their synchronization and lifetime.
type Input struct {
	values map[any]resolvedValue
}

func bindingValue[T any](input Input, binding any) T {
	resolved, exists := input.values[binding]
	if !exists {
		var zero T
		return zero
	}
	value, valid := resolved.value.(T)
	if !valid {
		var zero T
		return zero
	}

	return cloneValue(value)
}

func bindingState(input Input, binding any) ValueState {
	return input.values[binding].state
}

// Only the built-in collection contracts are copied. Arbitrary generic values
// retain their aliases rather than invoking reflection or application clone hooks.
func cloneValue[T any](value T) T {
	switch typed := any(value).(type) {
	case []string:
		return any(append([]string(nil), typed...)).(T)
	case map[string]string:
		clone := make(map[string]string, len(typed))
		for key, item := range typed {
			clone[key] = item
		}
		return any(clone).(T)
	default:
		return value
	}
}

func last(values []string) (string, error) {
	if len(values) == 0 {
		return "", fmt.Errorf("missing value")
	}

	return values[len(values)-1], nil
}

func parseString(values []string) (any, error) { return last(values) }

func parseBool(values []string) (any, error) {
	value, err := last(values)
	if err != nil {
		return nil, err
	}

	return strconv.ParseBool(value)
}

func parseInt(values []string) (any, error) {
	value, err := last(values)
	if err != nil {
		return nil, err
	}

	return strconv.ParseInt(value, 10, 64)
}

func parseUint(values []string) (any, error) {
	value, err := last(values)
	if err != nil {
		return nil, err
	}

	return strconv.ParseUint(value, 10, 64)
}

func parseFloat(values []string) (any, error) {
	value, err := last(values)
	if err != nil {
		return nil, err
	}

	return strconv.ParseFloat(value, 64)
}

func parseDuration(values []string) (any, error) {
	value, err := last(values)
	if err != nil {
		return nil, err
	}

	return time.ParseDuration(value)
}

func parseTime(layout string) func([]string) (any, error) {
	return func(values []string) (any, error) {
		value, err := last(values)
		if err != nil {
			return nil, err
		}

		return time.Parse(layout, value)
	}
}

func parseEnum(allowed []string) func([]string) (any, error) {
	values := append([]string(nil), allowed...)
	return func(raw []string) (any, error) {
		value, err := last(raw)
		if err != nil {
			return nil, err
		}
		for _, candidate := range values {
			if value == candidate {
				return value, nil
			}
		}

		return nil, fmt.Errorf("value is not in the allowed set")
	}
}

func parseStrings(values []string) (any, error) {
	return append([]string(nil), values...), nil
}

func parseKeyValues(values []string) (any, error) {
	result := make(map[string]string, len(values))
	for _, value := range values {
		key, item, found := strings.Cut(value, "=")
		if !found || key == "" {
			return nil, fmt.Errorf("expected key=value")
		}
		result[key] = item
	}

	return result, nil
}
