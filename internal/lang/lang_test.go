package lang

import "testing"

func TestByExtension(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     Language
	}{
		{name: "go", filename: "main.go", want: Language{ID: "go", Name: "Go"}},
		{name: "case insensitive", filename: "Main.GO", want: Language{ID: "go", Name: "Go"}},
		{name: "python", filename: "script.py", want: Language{ID: "python", Name: "Python"}},
		{name: "javascript module", filename: "index.mjs", want: Language{ID: "javascript", Name: "JavaScript"}},
		{name: "typescript", filename: "app.ts", want: Language{ID: "typescript", Name: "TypeScript"}},
		{name: "java", filename: "Main.java", want: Language{ID: "java", Name: "Java"}},
		{name: "c header", filename: "stdio.h", want: Language{ID: "c", Name: "C"}},
		{name: "cpp header", filename: "vector.hpp", want: Language{ID: "cpp", Name: "C++"}},
		{name: "rust", filename: "lib.rs", want: Language{ID: "rust", Name: "Rust"}},
		{name: "ruby", filename: "app.rb", want: Language{ID: "ruby", Name: "Ruby"}},
		{name: "html", filename: "index.htm", want: Language{ID: "html", Name: "HTML"}},
		{name: "css", filename: "styles.css", want: Language{ID: "css", Name: "CSS"}},
		{name: "json", filename: "package.json", want: Language{ID: "json", Name: "JSON"}},
		{name: "markdown", filename: "README.md", want: Language{ID: "markdown", Name: "Markdown"}},
		{name: "shell", filename: "run.sh", want: Language{ID: "shell", Name: "Shell"}},
		{name: "yaml", filename: "config.yaml", want: Language{ID: "yaml", Name: "YAML"}},
		{name: "unknown", filename: "notes.txt", want: Plaintext},
		{name: "empty", filename: "", want: Plaintext},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ByExtension(tt.filename); got != tt.want {
				t.Fatalf("ByExtension(%q) = %#v, want %#v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
		want     Language
	}{
		{
			name:     "filename preferred over content",
			filename: "main.go",
			content:  "#!/usr/bin/env python\nprint('hello')",
			want:     Language{ID: "go", Name: "Go"},
		},
		{
			name:    "python shebang with empty filename",
			content: "#!/usr/bin/env python\nprint('hello')",
			want:    Language{ID: "python", Name: "Python"},
		},
		{
			name:     "node shebang with unknown filename",
			filename: "script",
			content:  "#!/usr/bin/env node\nconsole.log('hello')",
			want:     Language{ID: "javascript", Name: "JavaScript"},
		},
		{
			name:    "bash shebang",
			content: "#!/bin/bash\necho hello",
			want:    Language{ID: "shell", Name: "Shell"},
		},
		{
			name:    "sh shebang",
			content: "#!/bin/sh\necho hello",
			want:    Language{ID: "shell", Name: "Shell"},
		},
		{
			name:    "ruby shebang",
			content: "#!/usr/bin/env ruby\nputs 'hello'",
			want:    Language{ID: "ruby", Name: "Ruby"},
		},
		{
			name:    "nothing resolves",
			content: "hello world",
			want:    Plaintext,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Detect(tt.filename, tt.content); got != tt.want {
				t.Fatalf("Detect(%q, %q) = %#v, want %#v", tt.filename, tt.content, got, tt.want)
			}
		})
	}
}

func TestAll(t *testing.T) {
	languages := All()
	if len(languages) == 0 {
		t.Fatal("All() returned no languages")
	}

	seen := make(map[string]bool, len(languages))
	for i, language := range languages {
		if language == Plaintext {
			t.Fatalf("All() includes Plaintext at index %d", i)
		}
		if language.ID == "" || language.Name == "" {
			t.Fatalf("All() contains incomplete language at index %d: %#v", i, language)
		}
		if seen[language.ID] {
			t.Fatalf("All() contains duplicate ID %q", language.ID)
		}
		seen[language.ID] = true
		if i > 0 && languages[i-1].ID > language.ID {
			t.Fatalf("All() not sorted by ID: %q before %q", languages[i-1].ID, language.ID)
		}
	}
}
