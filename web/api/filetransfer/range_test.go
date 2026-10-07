package filetransfer

import "testing"

func TestDownloadRanges(t *testing.T) {
	for _, tc := range []struct {
		header              string
		size, start, length int64
		invalid             bool
	}{
		{"bytes=134217728-", 256 << 20, 128 << 20, 128 << 20, false},
		{"bytes=-10", 100, 90, 10, false}, {"bytes=1-999", 10, 1, 9, false},
		{"bytes=0-0", 0, 0, 0, true}, {"bytes=10-", 10, 0, 0, true},
		{"bytes=0-1,3-4", 10, 0, 0, true}, {"bytes=2-1", 10, 0, 0, true},
	} {
		t.Run(tc.header, func(t *testing.T) {
			start, length, err := parseRange(tc.header, tc.size)
			if (err != nil) != tc.invalid || (!tc.invalid && (start != tc.start || length != tc.length)) {
				t.Fatalf("got %d,%d,%v", start, length, err)
			}
		})
	}
}
