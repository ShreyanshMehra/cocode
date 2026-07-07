// Package versiondiff provides line-based diffs between document versions.
package versiondiff

import (
	"strings"
)

// Kind identifies how a line participates in a diff.
type Kind int

const (
	// Equal marks a line present in both inputs.
	Equal Kind = iota
	// Insert marks a line present only in the new input.
	Insert
	// Delete marks a line present only in the old input.
	Delete
)

// String returns a readable name for the diff kind.
func (k Kind) String() string {
	switch k {
	case Equal:
		return "equal"
	case Insert:
		return "insert"
	case Delete:
		return "delete"
	default:
		return "unknown"
	}
}

// Line is one entry in a line-based edit script.
type Line struct {
	// Kind describes whether Text is equal, inserted, or deleted.
	Kind Kind
	// Text is the line content without a trailing newline.
	Text string
}

// Diff compares two texts line-by-line and returns the edit script that turns
// a into b. Equal lines are present in both inputs, Delete lines only in a,
// and Insert lines only in b.
func Diff(a, b string) []Line {
	oldLines := splitLines(a)
	newLines := splitLines(b)
	m, n := len(oldLines), len(newLines)

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	out := make([]Line, 0, m+n)
	for i, j := 0, 0; i < m || j < n; {
		switch {
		case i < m && j < n && oldLines[i] == newLines[j]:
			out = append(out, Line{Kind: Equal, Text: oldLines[i]})
			i++
			j++
		case i < m && (j == n || dp[i+1][j] >= dp[i][j+1]):
			out = append(out, Line{Kind: Delete, Text: oldLines[i]})
			i++
		case j < n:
			out = append(out, Line{Kind: Insert, Text: newLines[j]})
			j++
		}
	}

	return out
}

// Unified renders the diff with one prefixed, newline-terminated line per diff
// entry. Equal lines use " ", deletions use "-", and insertions use "+".
// It returns an empty string when there are no line changes.
func Unified(a, b string) string {
	lines := Diff(a, b)
	if allEqual(lines) {
		return ""
	}

	var builder strings.Builder
	for _, line := range lines {
		switch line.Kind {
		case Equal:
			builder.WriteByte(' ')
		case Insert:
			builder.WriteByte('+')
		case Delete:
			builder.WriteByte('-')
		}
		builder.WriteString(line.Text)
		builder.WriteByte('\n')
	}

	return builder.String()
}

func splitLines(text string) []string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func allEqual(lines []Line) bool {
	for _, line := range lines {
		if line.Kind != Equal {
			return false
		}
	}
	return true
}
