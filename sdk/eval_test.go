package sdk

import (
	"context"
	"reflect"
	"testing"
)

func TestEval(t *testing.T) {
	var tests = []struct {
		stmt     string
		expected any
	}{
		{
			stmt:     "3",
			expected: int64(3),
		},
		{
			stmt:     "1 + 2",
			expected: int64(3),
		},
		{
			stmt:     "'hello'",
			expected: "hello",
		},
		{
			stmt:     "[1,2,3]",
			expected: []any{int64(1), int64(2), int64(3)},
		},
		{
			stmt:     "{'key1': 'value1', 'key2': 'value2'}",
			expected: map[any]any{"key1": "value1", "key2": "value2"},
		},
	}

	backend := NewMockBackend()
	for _, test := range tests {
		t.Run(test.stmt, func(t *testing.T) {
			result, err := Eval(context.Background(), backend, "0.3.0", test.stmt)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("expected %v (%T), got %v (%T)", test.expected, test.expected, result, result)
			}
		})
	}
}
