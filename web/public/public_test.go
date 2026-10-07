package public

import (
	"io/fs"
	"testing"
)

func TestBuiltinThemesAreEmbedded(t *testing.T) {
	entries, err := fs.ReadDir(PublicFS, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "defaultTheme" || entries[1].Name() != "retroTheme" {
		t.Fatalf("unexpected bundled themes: %v", entries)
	}
	for _, theme := range []struct {
		id   string
		root string
	}{{DefaultTheme, "defaultTheme"}, {RetroTheme, "retroTheme"}} {
		if _, err := fs.Stat(PublicFS, theme.root+"/dist/index.html"); err != nil {
			t.Fatalf("%s theme missing: %v", theme.id, err)
		}
		if _, ok := ReadBuiltinThemeFile(theme.id, "komari-theme.json"); !ok {
			t.Fatalf("%s theme metadata missing", theme.id)
		}
	}
}

func TestReadBuiltinThemeFileRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../defaultTheme/komari-theme.json", `..\defaultTheme\komari-theme.json`} {
		if _, ok := ReadBuiltinThemeFile(RetroTheme, name); ok {
			t.Fatalf("unexpectedly read path outside the embedded theme: %q", name)
		}
	}
}

func TestNormalizeHTMLLanguage(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"hyphen language": {
			input: "zh-CN",
			want:  "zh-CN",
		},
		"underscore language": {
			input: "zh_CN",
			want:  "zh-CN",
		},
		"reject script injection": {
			input: `zh-CN" autofocus`,
		},
		"reject too short": {
			input: "z",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := normalizeHTMLLanguage(tt.input); got != tt.want {
				t.Fatalf("normalizeHTMLLanguage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReplaceHTMLLanguage(t *testing.T) {
	tests := map[string]struct {
		html     string
		language string
		want     string
	}{
		"replace existing lang": {
			html:     `<html lang="en"><head></head></html>`,
			language: "zh-CN",
			want:     `<html lang="zh-CN"><head></head></html>`,
		},
		"insert missing lang": {
			html:     `<html><head></head></html>`,
			language: "ja_JP",
			want:     `<html lang="ja-JP"><head></head></html>`,
		},
		"ignore invalid lang": {
			html:     `<html lang="en"><head></head></html>`,
			language: `zh-CN" autofocus`,
			want:     `<html lang="en"><head></head></html>`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := replaceHTMLLanguage(tt.html, tt.language); got != tt.want {
				t.Fatalf("replaceHTMLLanguage() = %q, want %q", got, tt.want)
			}
		})
	}
}
