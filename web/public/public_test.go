package public

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMissingAPIRoutesDoNotServeFrontend(t *testing.T) {
	r := gin.New()
	r.GET("/api/known", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	Static(r.Group("/"), r.NoRoute)
	for _, endpoint := range []string{"/api", "/api/missing", "/api/admin/2fa/generate", "/api/admin/2fa/enable"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, httptest.NewRequest(method, endpoint, nil))
			if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "<html") {
				t.Fatalf("%s %s returned %d with frontend content", method, endpoint, recorder.Code)
			}
		}
	}
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/known", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("registered API stopped working: %d %s", recorder.Code, recorder.Body.String())
	}
}

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

func TestRetroThemeEmbedsEveryNextAssetReferencedByIndex(t *testing.T) {
	index, ok := ReadBuiltinThemeFile(RetroTheme, "dist/index.html")
	if !ok {
		t.Fatal("retro theme index is missing")
	}

	if strings.Contains(string(index), `"/_next/static/`) {
		t.Fatal("retro theme index still references Next.js assets from the root path")
	}

	assetReferences := regexp.MustCompile(`(src|href)="(/private-assets/retro/_next/static/[^"]+)"`).FindAllSubmatch(index, -1)
	if len(assetReferences) == 0 {
		t.Fatal("retro theme index does not reference any Next.js assets")
	}

	for _, reference := range assetReferences {
		assetPath := path.Join("dist", strings.TrimPrefix(string(reference[2]), "/private-assets/retro/"))
		if _, ok := ReadBuiltinThemeFile(RetroTheme, assetPath); !ok {
			t.Errorf("retro theme index references an asset that is not embedded: %s", assetPath)
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
