package cc2butane

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/ayush-that/cloud-config-to-butane/internal/transpile"
)

var update = flag.Bool("update", false, "regenerate golden .butane.yaml files")

// convertible fixtures produce Butane and have a golden output.
var convertible = []string{
	"01-write-files",
	"02-users-groups",
	"03-runcmd",
	"04-ca-certs",
	"05-capi-worker",
	"08-gzip-base64",
	"09-user-passwd",
}

// assertRoundTrips feeds emitted Butane to Butane's own translator and fails if it errors or the report is fatal.
func assertRoundTrips(t *testing.T, butane []byte) {
	t.Helper()
	if err := transpile.Validate(butane); err != nil {
		t.Fatalf("emitted butane did not round-trip through butane: %v\n--- butane ---\n%s", err, butane)
	}
}

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return b
}

func TestGolden(t *testing.T) {
	for _, name := range convertible {
		t.Run(name, func(t *testing.T) {
			in := read(t, filepath.Join("testdata", name+".yaml"))
			got, err := transpile.Transpile(in, transpile.Options{})
			if err != nil {
				t.Fatalf("transpile: %v", err)
			}

			goldenPath := filepath.Join("testdata", name+".butane.yaml")
			if *update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
			}

			want := read(t, goldenPath)
			if !bytes.Equal(got, want) {
				t.Errorf("output does not match golden %s (run: go test ./... -update)\n--- got ---\n%s\n--- want ---\n%s",
					goldenPath, got, want)
			}

			assertRoundTrips(t, got)
		})
	}
}

func TestUnsupportedPackages(t *testing.T) {
	in := read(t, filepath.Join("testdata", "06-packages.yaml"))

	_, err := transpile.Transpile(in, transpile.Options{})
	var ue *transpile.UnsupportedError
	if !errors.As(err, &ue) {
		t.Fatalf("expected UnsupportedError, got %v", err)
	}

	// With WarnUnsupported the key is dropped and the rest still transpiles.
	out, err := transpile.Transpile(in, transpile.Options{WarnUnsupported: true})
	if err != nil {
		t.Fatalf("warn-unsupported should not error: %v", err)
	}
	assertRoundTrips(t, out)
}

func TestRejectedJinja(t *testing.T) {
	in := read(t, filepath.Join("testdata", "07-jinja.yaml"))
	_, err := transpile.Transpile(in, transpile.Options{})
	var re *transpile.RejectedError
	if !errors.As(err, &re) {
		t.Fatalf("expected RejectedError, got %v", err)
	}
}

func TestStrictRejectsRuncmd(t *testing.T) {
	in := read(t, filepath.Join("testdata", "03-runcmd.yaml"))
	_, err := transpile.Transpile(in, transpile.Options{Strict: true})
	var ue *transpile.UnsupportedError
	if !errors.As(err, &ue) {
		t.Fatalf("expected UnsupportedError under --strict, got %v", err)
	}
}
