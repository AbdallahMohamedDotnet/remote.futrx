package output

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		value string
		max   int
		want  string
	}{
		{name: "shorter", value: "abc", max: 4, want: "abc"},
		{name: "exact", value: "abcd", max: 4, want: "abcd"},
		{name: "longer", value: "abcde", max: 4, want: "abcd..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.value, tt.max); got != tt.want {
				t.Fatalf("Truncate(%q, %d) = %q, want %q", tt.value, tt.max, got, tt.want)
			}
		})
	}
}
