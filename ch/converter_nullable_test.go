package ch

import "testing"

// TestParseColumnType_NullableWrappers verifies that the IsNullable flag and the
// unwrapped BaseType/ParsedType are resolved correctly regardless of how the
// Nullable modifier is wrapped. This is a regression test for the bug where
// LowCardinality(Nullable(String)) was treated as non-nullable (only the bare
// "Nullable(" prefix was checked), so a missing value was written as "" instead
// of NULL.
func TestParseColumnType_NullableWrappers(t *testing.T) {
	tests := []struct {
		name         string
		colType      string
		wantNullable bool
		wantType     ClickhouseType
	}{
		{"plain-string", "String", false, TypeString},
		{"nullable-string", "Nullable(String)", true, TypeString},
		{"lowcardinality-string", "LowCardinality(String)", false, TypeString},
		{"lowcardinality-nullable-string", "LowCardinality(Nullable(String))", true, TypeString},
		{"lowcardinality-nullable-int32", "LowCardinality(Nullable(Int32))", true, TypeInt},
		{"nullable-int64", "Nullable(Int64)", true, TypeInt},
		{"nullable-float64", "Nullable(Float64)", true, TypeFloat},
		{"nullable-bool", "Nullable(Bool)", true, TypeBool},
		{"nullable-datetime64", "Nullable(DateTime64(3))", true, TypeDateTime},
		{"nullable-decimal", "Nullable(Decimal(10, 2))", true, TypeDecimal},
		{"lc-nullable-fixedstring", "LowCardinality(Nullable(FixedString(10)))", true, TypeString},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := parseColumnType("c", tt.colType, "")
			if col.IsNullable != tt.wantNullable {
				t.Fatalf("IsNullable = %v, want %v (type %q)", col.IsNullable, tt.wantNullable, tt.colType)
			}
			if col.ParsedType != tt.wantType {
				t.Fatalf("ParsedType = %v, want %v (type %q)", col.ParsedType, tt.wantType, tt.colType)
			}
		})
	}
}

// TestGetZeroValue_NullableWrappers verifies that an absent value for any nullable
// column is written as NULL (nil), not the type's zero value. The
// LowCardinality(Nullable(...)) cases are the regression: previously they returned
// "" / 0 because the column was mis-detected as non-nullable.
func TestGetZeroValue_NullableWrappers(t *testing.T) {
	nullableTypes := []string{
		"Nullable(String)",
		"LowCardinality(Nullable(String))",
		"LowCardinality(Nullable(Int32))",
		"Nullable(Int64)",
		"Nullable(DateTime64(3))",
	}
	for _, ct := range nullableTypes {
		t.Run(ct, func(t *testing.T) {
			col := parseColumnType("c", ct, "")
			if zv := getZeroValue(&col); zv != nil {
				t.Fatalf("getZeroValue(%q) = %#v, want nil (NULL)", ct, zv)
			}
		})
	}

	// Non-nullable counterparts must still fall back to their typed zero value.
	col := parseColumnType("c", "LowCardinality(String)", "")
	if zv := getZeroValue(&col); zv != "" {
		t.Fatalf("getZeroValue(LowCardinality(String)) = %#v, want \"\"", zv)
	}
}
