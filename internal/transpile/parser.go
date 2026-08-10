package transpile

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type Options struct {
	Strict          bool
	WarnUnsupported bool
	Warn            io.Writer
}

type UnsupportedError struct {
	Key    string
	Detail string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("unsupported key %q: %s", e.Key, e.Detail)
}

type RejectedError struct {
	Reason string
}

func (e *RejectedError) Error() string { return "rejected input: " + e.Reason }

func parse(raw []byte) (map[string]any, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing cloud-config yaml: %w", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}
