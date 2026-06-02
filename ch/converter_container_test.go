package ch

import (
	"reflect"
	"testing"
)

// TestParseColumnType_Map verifies Map(...) columns are classified as TypeMap
// and resolved to the concrete Go map type the driver expects. This is a
// regression test for the bug where Map(String, String) was misclassified as
// TypeString (because the type string contains the "STRING" substring),
// causing batch inserts to fail with
// "converting string to Map(String, String) is unsupported".
func TestParseColumnType_Map(t *testing.T) {
	tests := []struct {
		name         string
		colType      string
		wantScanType reflect.Type
	}{
		{"string-string", "Map(String, String)", reflect.TypeOf(map[string]string(nil))},
		{"string-int64", "Map(String, Int64)", reflect.TypeOf(map[string]int64(nil))},
		{"string-int32", "Map(String, Int32)", reflect.TypeOf(map[string]int32(nil))},
		{"string-float64", "Map(String, Float64)", reflect.TypeOf(map[string]float64(nil))},
		{"lowcardinality-key", "Map(LowCardinality(String), String)", reflect.TypeOf(map[string]string(nil))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := parseColumnType("m", tt.colType, "")
			if col.ParsedType != TypeMap {
				t.Fatalf("ParsedType = %v, want TypeMap", col.ParsedType)
			}
			if col.ContainerScanType != tt.wantScanType {
				t.Fatalf("ContainerScanType = %v, want %v", col.ContainerScanType, tt.wantScanType)
			}
		})
	}
}

// TestGetZeroValue_Map verifies the zero value for a Map column is an empty map
// of the exact driver-expected type (not nil, not "").
func TestGetZeroValue_Map(t *testing.T) {
	col := parseColumnType("m", "Map(String, String)", "")
	zv := getZeroValue(&col)
	m, ok := zv.(map[string]string)
	if !ok {
		t.Fatalf("zero value type = %T, want map[string]string", zv)
	}
	if len(m) != 0 {
		t.Fatalf("zero value len = %d, want 0", len(m))
	}
}

func TestMapConverter_StringString(t *testing.T) {
	col := parseColumnType("m", "Map(String, String)", "")
	conv := getConverter(&col)

	tests := []struct {
		name string
		in   any
		want map[string]string
	}{
		{"map[string]any", map[string]any{"a": "1", "b": "2"}, map[string]string{"a": "1", "b": "2"}},
		{"map[string]string passthrough", map[string]string{"x": "y"}, map[string]string{"x": "y"}},
		{"numeric values coerced", map[string]any{"k": int64(7)}, map[string]string{"k": "7"}},
		{"json string", `{"p":"q","r":"s"}`, map[string]string{"p": "q", "r": "s"}},
		{"json bytes", []byte(`{"u":"v"}`), map[string]string{"u": "v"}},
		{"nil -> empty", nil, map[string]string{}},
		{"non-map -> empty", 42, map[string]string{}},
		{"invalid json -> empty", "not json", map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := conv.Convert(tt.in, nil)
			if err != nil {
				t.Fatalf("Convert error: %v", err)
			}
			m, ok := got.(map[string]string)
			if !ok {
				t.Fatalf("result type = %T, want map[string]string", got)
			}
			if !reflect.DeepEqual(m, tt.want) {
				t.Fatalf("result = %v, want %v", m, tt.want)
			}
		})
	}
}

// TestMapConverter_StringInt64 verifies value-type coercion produces the exact
// map[string]int64 the driver requires, parsing both numbers and numeric strings.
func TestMapConverter_StringInt64(t *testing.T) {
	col := parseColumnType("m", "Map(String, Int64)", "")
	conv := getConverter(&col)

	got, err := conv.Convert(map[string]any{"a": float64(3), "b": "5"}, nil)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	m, ok := got.(map[string]int64)
	if !ok {
		t.Fatalf("result type = %T, want map[string]int64", got)
	}
	want := map[string]int64{"a": 3, "b": 5}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("result = %v, want %v", m, want)
	}
}

// TestMapConverter_StringInt32 verifies narrowing coercion to a non-canonical
// integer width (int64 -> int32) yields the exact map[string]int32 type.
func TestMapConverter_StringInt32(t *testing.T) {
	col := parseColumnType("m", "Map(String, Int32)", "")
	conv := getConverter(&col)

	got, err := conv.Convert(map[string]any{"a": int64(9)}, nil)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if _, ok := got.(map[string]int32); !ok {
		t.Fatalf("result type = %T, want map[string]int32", got)
	}
}

// TestMapConverter_NilScanType ensures an unresolved Map type degrades safely
// to nil rather than panicking.
func TestMapConverter_NilScanType(t *testing.T) {
	conv := &MapConverter{ScanType: nil}
	got, err := conv.Convert(map[string]any{"a": "b"}, nil)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if got != nil {
		t.Fatalf("result = %v, want nil", got)
	}
}

// TestParseColumnType_Array verifies Array(...) columns are classified as TypeArray
// and resolved to the concrete Go slice type the driver expects. Regression test
// for the same substring-misclassification bug as Map (Array(String) contains
// "STRING", Array(Int64) contains "INT").
func TestParseColumnType_Array(t *testing.T) {
	tests := []struct {
		name         string
		colType      string
		wantScanType reflect.Type
	}{
		{"string", "Array(String)", reflect.TypeOf([]string(nil))},
		{"int64", "Array(Int64)", reflect.TypeOf([]int64(nil))},
		{"float64", "Array(Float64)", reflect.TypeOf([]float64(nil))},
		{"lowcardinality", "Array(LowCardinality(String))", reflect.TypeOf([]string(nil))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := parseColumnType("a", tt.colType, "")
			if col.ParsedType != TypeArray {
				t.Fatalf("ParsedType = %v, want TypeArray", col.ParsedType)
			}
			if col.ContainerScanType != tt.wantScanType {
				t.Fatalf("ContainerScanType = %v, want %v", col.ContainerScanType, tt.wantScanType)
			}
		})
	}
}

// TestGetZeroValue_Array verifies the zero value for an Array column is an empty
// (non-nil) slice of the exact driver-expected type.
func TestGetZeroValue_Array(t *testing.T) {
	col := parseColumnType("a", "Array(String)", "")
	zv := getZeroValue(&col)
	s, ok := zv.([]string)
	if !ok {
		t.Fatalf("zero value type = %T, want []string", zv)
	}
	if s == nil || len(s) != 0 {
		t.Fatalf("zero value = %#v, want empty non-nil []string", s)
	}
}

func TestArrayConverter_String(t *testing.T) {
	col := parseColumnType("a", "Array(String)", "")
	conv := getConverter(&col)

	tests := []struct {
		name string
		in   any
		want []string
	}{
		{"[]any", []any{"a", "b"}, []string{"a", "b"}},
		{"[]string passthrough", []string{"x", "y"}, []string{"x", "y"}},
		{"numeric elems coerced", []any{int64(1), float64(2)}, []string{"1", "2"}},
		{"json string", `["p","q"]`, []string{"p", "q"}},
		{"nil -> empty", nil, []string{}},
		{"non-array -> empty", "plain", []string{}},
		{"empty array", []any{}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := conv.Convert(tt.in, nil)
			if err != nil {
				t.Fatalf("Convert error: %v", err)
			}
			s, ok := got.([]string)
			if !ok {
				t.Fatalf("result type = %T, want []string", got)
			}
			if !reflect.DeepEqual(s, tt.want) {
				t.Fatalf("result = %#v, want %#v", s, tt.want)
			}
		})
	}
}

// TestArrayConverter_Int64 verifies element coercion to the exact []int64 the
// driver requires, parsing both numbers and numeric strings.
func TestArrayConverter_Int64(t *testing.T) {
	col := parseColumnType("a", "Array(Int64)", "")
	conv := getConverter(&col)

	got, err := conv.Convert([]any{float64(3), "5"}, nil)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	s, ok := got.([]int64)
	if !ok {
		t.Fatalf("result type = %T, want []int64", got)
	}
	if want := []int64{3, 5}; !reflect.DeepEqual(s, want) {
		t.Fatalf("result = %v, want %v", s, want)
	}
}

// TestMapConverter_ArrayValue verifies a nested container (Map with Array values)
// builds the exact map[string][]string the driver requires.
func TestMapConverter_ArrayValue(t *testing.T) {
	col := parseColumnType("m", "Map(String, Array(String))", "")
	conv := getConverter(&col)

	got, err := conv.Convert(map[string]any{"k": []any{"a", "b"}}, nil)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	m, ok := got.(map[string][]string)
	if !ok {
		t.Fatalf("result type = %T, want map[string][]string", got)
	}
	if want := map[string][]string{"k": {"a", "b"}}; !reflect.DeepEqual(m, want) {
		t.Fatalf("result = %v, want %v", m, want)
	}
}
