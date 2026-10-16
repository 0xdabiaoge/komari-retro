package cmd

import "testing"

func TestFleetConnectionLimitRemainsBounded(t *testing.T) {
	for _, test := range []struct {
		input string
		want  int
	}{{"", 2048}, {"512", 512}, {"8192", 8192}, {"0", 2048}, {"-1", 2048}, {"999999", 2048}, {"invalid", 2048}} {
		t.Setenv("KOMARI_MAX_CONNECTIONS", test.input)
		if got := serverConnectionLimit(); got != test.want {
			t.Fatalf("%q: %d, want %d", test.input, got, test.want)
		}
	}
}
