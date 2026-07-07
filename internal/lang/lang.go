// Package lang detects programming languages from document names and content.
package lang

import (
	"path/filepath"
	"sort"
	"strings"
)

// Language describes a programming language for syntax highlighting.
type Language struct {
	ID   string `json:"id"`   // short id, e.g. "go", "python", "javascript", "plaintext"
	Name string `json:"name"` // display name, e.g. "Go", "Python", "JavaScript", "Plain Text"
}

// Plaintext is the fallback language.
var Plaintext = Language{ID: "plaintext", Name: "Plain Text"}

var languagesByExtension = map[string]Language{
	".c":    {ID: "c", Name: "C"},
	".cc":   {ID: "cpp", Name: "C++"},
	".cpp":  {ID: "cpp", Name: "C++"},
	".css":  {ID: "css", Name: "CSS"},
	".go":   {ID: "go", Name: "Go"},
	".h":    {ID: "c", Name: "C"},
	".htm":  {ID: "html", Name: "HTML"},
	".html": {ID: "html", Name: "HTML"},
	".hpp":  {ID: "cpp", Name: "C++"},
	".java": {ID: "java", Name: "Java"},
	".js":   {ID: "javascript", Name: "JavaScript"},
	".json": {ID: "json", Name: "JSON"},
	".md":   {ID: "markdown", Name: "Markdown"},
	".mjs":  {ID: "javascript", Name: "JavaScript"},
	".py":   {ID: "python", Name: "Python"},
	".rb":   {ID: "ruby", Name: "Ruby"},
	".rs":   {ID: "rust", Name: "Rust"},
	".sh":   {ID: "shell", Name: "Shell"},
	".ts":   {ID: "typescript", Name: "TypeScript"},
	".yaml": {ID: "yaml", Name: "YAML"},
	".yml":  {ID: "yaml", Name: "YAML"},
}

// ByExtension returns the language for a filename based on its extension.
// Unknown or empty filenames return Plaintext.
func ByExtension(filename string) Language {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return Plaintext
	}
	if language, ok := languagesByExtension[ext]; ok {
		return language
	}
	return Plaintext
}

// Detect determines the language from a filename first; if the filename is
// empty or unknown, it falls back to content heuristics such as shebang lines.
// Plaintext is returned when neither filename nor content matches.
func Detect(filename, content string) Language {
	if language := ByExtension(filename); language != Plaintext {
		return language
	}

	firstLine := content
	if i := strings.IndexAny(content, "\r\n"); i >= 0 {
		firstLine = content[:i]
	}
	if !strings.HasPrefix(firstLine, "#!") {
		return Plaintext
	}

	shebang := strings.ToLower(firstLine)
	switch {
	case strings.Contains(shebang, "python"):
		return Language{ID: "python", Name: "Python"}
	case strings.Contains(shebang, "node"):
		return Language{ID: "javascript", Name: "JavaScript"}
	case strings.Contains(shebang, "bash"), strings.Contains(shebang, "sh"):
		return Language{ID: "shell", Name: "Shell"}
	case strings.Contains(shebang, "ruby"):
		return Language{ID: "ruby", Name: "Ruby"}
	default:
		return Plaintext
	}
}

// All returns every known language except Plaintext, sorted by ID.
func All() []Language {
	byID := make(map[string]Language, len(languagesByExtension))
	for _, language := range languagesByExtension {
		byID[language.ID] = language
	}

	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	languages := make([]Language, 0, len(ids))
	for _, id := range ids {
		languages = append(languages, byID[id])
	}
	return languages
}
