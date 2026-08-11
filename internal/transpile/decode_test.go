package transpile

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"testing"
)

func gzB64(t *testing.T, s string) string {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestDecodeContent(t *testing.T) {
	const want = "kernel.pid_max = 4194304\n"
	cases := []struct{ encoding, content string }{
		{"", want},
		{"text/plain", want},
		{"b64", base64.StdEncoding.EncodeToString([]byte(want))},
		{"base64", base64.StdEncoding.EncodeToString([]byte(want))},
		{"gz+b64", gzB64(t, want)},
		{"gzip+base64", gzB64(t, want)},
	}
	for _, c := range cases {
		got, err := decodeContent(c.content, c.encoding)
		if err != nil {
			t.Fatalf("encoding %q: %v", c.encoding, err)
		}
		if string(got) != want {
			t.Errorf("encoding %q: got %q, want %q", c.encoding, got, want)
		}
	}
}

func TestShellifyBody(t *testing.T) {
	got, err := shellifyBody([]any{
		[]any{"sh", "-c", "echo 'hi' > /x"},
		"raw --line untouched",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "'sh' '-c' 'echo '\\''hi'\\'' > /x'\nraw --line untouched\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestDecodePermissions(t *testing.T) {
	cases := []struct {
		in   any
		want int
	}{
		{nil, 0o644},
		{"0600", 0o600},
		{"644", 0o644},
		{420, 420}, // yaml resolves 0644 to decimal 420 before we see it
	}
	for _, c := range cases {
		got, has, err := decodePermissions(c.in)
		if err != nil || !has {
			t.Fatalf("decodePermissions(%v): has=%v err=%v", c.in, has, err)
		}
		if got != c.want {
			t.Errorf("decodePermissions(%v) = %o, want %o", c.in, got, c.want)
		}
	}
}
