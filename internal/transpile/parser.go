package transpile

import (
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// Options controls how unsupported and lossy keys are handled.
type Options struct {
	// Strict turns runcmd and bootcmd into hard errors instead of systemd units.
	Strict bool
	// WarnUnsupported downgrades unsupported package keys from a hard error to a warning and drops them.
	WarnUnsupported bool
	// Warn receives one warning line per dropped key; nil discards warnings.
	Warn io.Writer
}

// UnsupportedError is returned for cloud-config keys with no honest immutable-OS equivalent.
type UnsupportedError struct {
	Key    string
	Detail string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("unsupported key %q: %s", e.Key, e.Detail)
}

// RejectedError is returned when the input is not a plain #cloud-config document.
type RejectedError struct {
	Reason string
}

func (e *RejectedError) Error() string { return "rejected input: " + e.Reason }

// detect rejects input that is not a plain #cloud-config, judging only the leading line as cloud-init does.
func detect(raw []byte) error {
	s := strings.TrimLeft(string(raw), "\uFEFF")
	var first string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			first = strings.TrimRight(line, " \t\r")
			break
		}
	}
	trimmed := strings.TrimSpace(first)

	switch {
	case strings.HasPrefix(trimmed, "## template: jinja"):
		return &RejectedError{Reason: "jinja-templated cloud-config (## template: jinja); render the template before transpiling"}
	case strings.HasPrefix(trimmed, "#include"):
		return &RejectedError{Reason: "#include / #include-once directive; only a self-contained #cloud-config is supported"}
	case strings.HasPrefix(trimmed, "#cloud-config-archive"), strings.HasPrefix(trimmed, "#cloud-config-jsonp"):
		return &RejectedError{Reason: "cloud-config archive/jsonp; only a plain #cloud-config is supported"}
	case strings.HasPrefix(trimmed, "#cloud-boothook"), strings.HasPrefix(trimmed, "#!"):
		return &RejectedError{Reason: "boothook or shell script user-data; only a plain #cloud-config is supported"}
	case strings.HasPrefix(trimmed, "Content-Type: multipart"), strings.HasPrefix(trimmed, "MIME-Version:"):
		return &RejectedError{Reason: "MIME multipart user-data; only a plain #cloud-config is supported"}
	}

	// Valid header is "#cloud-config" optionally followed by whitespace and a comment; anything else is refused.
	rest, ok := strings.CutPrefix(trimmed, "#cloud-config")
	if !ok || (rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\t")) {
		return &RejectedError{Reason: "missing #cloud-config header on the first line"}
	}
	return nil
}

func parse(raw []byte) (map[string]any, error) {
	if err := detect(raw); err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing cloud-config yaml: %w", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}
