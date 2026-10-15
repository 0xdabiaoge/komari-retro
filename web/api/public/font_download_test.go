package public

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/image/font/gofont/goregular"
)

func TestFontDownloadFetchesOnlyRequiredArchiveRanges(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	padding, err := z.CreateHeader(&zip.FileHeader{Name: "unused.bin", Method: zip.Store})
	require.NoError(t, err)
	_, err = padding.Write(make([]byte, 8<<20))
	require.NoError(t, err)
	font, err := z.Create(fontZipEntry)
	require.NoError(t, err)
	_, err = font.Write(goregular.TTF)
	require.NoError(t, err)
	require.NoError(t, z.Close())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "fonts.zip", time.Time{}, bytes.NewReader(buf.Bytes()))
	}))
	defer server.Close()
	var fetched int64
	data, err := downloadFontData(context.Background(), server.URL, func(n int64) { fetched += n })
	require.NoError(t, err)
	require.Equal(t, goregular.TTF, data)
	require.Less(t, fetched, int64(buf.Len()/2))
	path := filepath.Join(t.TempDir(), "font.ttf")
	require.NoError(t, saveDownloadedFont(path, data))
	require.Error(t, saveDownloadedFont(path, []byte("corrupted download")))
	saved, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, goregular.TTF, saved)
}

func TestFontDownloadRejectsUnboundedAndInvalidResponses(t *testing.T) {
	for _, handler := range []http.HandlerFunc{
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) },
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Range", "bytes 0-1048575/9999999999")
			w.WriteHeader(206)
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Range", "bytes 0-9/10")
			w.WriteHeader(206)
			_, _ = w.Write([]byte("not a ZIP!"))
		},
	} {
		server := httptest.NewServer(handler)
		_, err := downloadFontData(context.Background(), server.URL, nil)
		require.Error(t, err)
		server.Close()
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := downloadFontData(ctx, server.URL, nil)
	require.Error(t, err)
}

func TestFontDownloadRejectsOversizedUncompressedFontBeforeReadingIt(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	file, err := z.Create(fontZipEntry)
	require.NoError(t, err)
	_, err = file.Write([]byte("small fixture"))
	require.NoError(t, err)
	require.NoError(t, z.Close())
	data := buf.Bytes()
	central := bytes.Index(data, []byte{'P', 'K', 1, 2})
	require.NotEqual(t, -1, central)
	binary.LittleEndian.PutUint32(data[central+24:], uint32(maxFontBytes+1))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "fixture.zip", time.Time{}, bytes.NewReader(data))
	}))
	defer server.Close()
	_, err = downloadFontData(context.Background(), server.URL, nil)
	require.ErrorContains(t, err, "32 MiB")
}
