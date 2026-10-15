package public

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font/opentype"
)

const fontChunkSize int64 = 1 << 20
const maxFontDownloadBytes int64 = 64 << 20
const maxFontBytes int64 = 32 << 20

// ZIP's central directory and the requested TTF can be read independently.
// The vendor archive is much larger than the single font needed by MJPEG.
type fontRangeReader struct {
	ctx      context.Context
	url      string
	size     int64
	chunks   map[int64][]byte
	bytes    int64
	progress func(int64)
	mu       sync.Mutex
}

func (r *fontRangeReader) chunk(start int64) ([]byte, error) {
	if body, ok := r.chunks[start]; ok {
		return body, nil
	}
	end := start + fontChunkSize - 1
	if r.size > 0 && end >= r.size {
		end = r.size - 1
	}
	if r.bytes+end-start+1 > maxFontDownloadBytes {
		return nil, fmt.Errorf("font download exceeds 64 MiB budget")
	}
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("font server must support range requests: HTTP %d", resp.StatusCode)
	}
	var actualStart, actualEnd, total int64
	if _, err := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes %d-%d/%d", &actualStart, &actualEnd, &total); err != nil || actualStart != start || actualEnd < start || actualEnd > end || total <= actualEnd || total > 512<<20 || (r.size > 0 && total != r.size) {
		return nil, fmt.Errorf("invalid font archive range")
	}
	if actualEnd != end && actualEnd != total-1 {
		return nil, fmt.Errorf("incomplete font archive range")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, fontChunkSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) != actualEnd-start+1 {
		return nil, fmt.Errorf("font archive range length mismatch")
	}
	r.size = total
	r.bytes += int64(len(body))
	r.chunks[start] = body
	if r.progress != nil {
		r.progress(int64(len(body)))
	}
	return body, nil
}

func (r *fontRangeReader) ReadAt(p []byte, offset int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if offset < 0 {
		return 0, fmt.Errorf("invalid font archive offset")
	}
	n := 0
	for n < len(p) {
		if offset >= r.size {
			return n, io.EOF
		}
		start := offset / fontChunkSize * fontChunkSize
		body, err := r.chunk(start)
		if err != nil {
			return n, err
		}
		copied := copy(p[n:], body[offset-start:])
		n += copied
		offset += int64(copied)
	}
	return n, nil
}

func downloadFontData(ctx context.Context, url string, progress func(int64)) ([]byte, error) {
	reader := &fontRangeReader{ctx: ctx, url: url, chunks: make(map[int64][]byte), progress: progress}
	if _, err := reader.chunk(0); err != nil {
		return nil, err
	}
	archive, err := zip.NewReader(reader, reader.size)
	if err != nil {
		return nil, err
	}
	for _, file := range archive.File {
		if file.Name != fontZipEntry && !strings.HasSuffix(file.Name, "MiSans-Normal.ttf") {
			continue
		}
		if file.UncompressedSize64 > uint64(maxFontBytes) || file.CompressedSize64 > uint64(maxFontBytes) {
			return nil, fmt.Errorf("font exceeds 32 MiB limit")
		}
		body, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer body.Close()
		data, err := io.ReadAll(io.LimitReader(body, maxFontBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxFontBytes {
			return nil, fmt.Errorf("font exceeds 32 MiB limit")
		}
		return data, nil
	}
	return nil, fmt.Errorf("font not found in archive")
}

func saveDownloadedFont(path string, data []byte) error {
	if _, err := opentype.Parse(data); err != nil {
		return fmt.Errorf("invalid downloaded font: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".font-ready-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
