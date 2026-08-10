package cc2butane

import (
	"bytes"
	"os"
	"testing"

	"github.com/ayush-that/cloud-config-to-butane/internal/transpile"
)

func TestWriteFiles(t *testing.T) {
	in, err := os.ReadFile("testdata/01-write-files.yaml")
	if err != nil {
		t.Fatal(err)
	}
	got, err := transpile.Transpile(in, transpile.Options{})
	if err != nil {
		t.Fatalf("transpile: %v", err)
	}
	want, err := os.ReadFile("testdata/01-write-files.butane.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
