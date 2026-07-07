package versiondiff

import (
	"reflect"
	"testing"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want []Line
	}{
		{
			name: "identical",
			a:    "one\ntwo",
			b:    "one\ntwo",
			want: []Line{
				{Kind: Equal, Text: "one"},
				{Kind: Equal, Text: "two"},
			},
		},
		{
			name: "pure insertion",
			a:    "",
			b:    "x\ny",
			want: []Line{
				{Kind: Insert, Text: "x"},
				{Kind: Insert, Text: "y"},
			},
		},
		{
			name: "pure deletion",
			a:    "x\ny",
			b:    "",
			want: []Line{
				{Kind: Delete, Text: "x"},
				{Kind: Delete, Text: "y"},
			},
		},
		{
			name: "middle modification",
			a:    "one\ntwo\nthree",
			b:    "one\n2\nthree",
			want: []Line{
				{Kind: Equal, Text: "one"},
				{Kind: Delete, Text: "two"},
				{Kind: Insert, Text: "2"},
				{Kind: Equal, Text: "three"},
			},
		},
		{
			name: "trailing newline",
			a:    "a\n",
			b:    "a\n",
			want: []Line{{Kind: Equal, Text: "a"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Diff(tt.a, tt.b)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Diff(%q, %q) = %#v, want %#v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestUnified(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{
			name: "identical",
			a:    "one\ntwo",
			b:    "one\ntwo",
			want: "",
		},
		{
			name: "format",
			a:    "one\ntwo",
			b:    "one\n2",
			want: " one\n-two\n+2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Unified(tt.a, tt.b)
			if got != tt.want {
				t.Fatalf("Unified(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
