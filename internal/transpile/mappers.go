package transpile

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/vincent-petithory/dataurl"
)

func mapWriteFiles(v any, doc *document) error {
	items, ok := v.([]any)
	if !ok {
		return fmt.Errorf("write_files must be a list, got %T", v)
	}
	for i, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("write_files[%d] must be a mapping, got %T", i, raw)
		}
		path, _ := toString(m["path"])
		if path == "" {
			return fmt.Errorf("write_files[%d] is missing a path", i)
		}
		f := file{path: path, headComment: "from write_files"}

		content, _ := toString(m["content"])
		encoding, _ := toString(m["encoding"])
		decoded, err := decodeContent(content, encoding)
		if err != nil {
			return fmt.Errorf("write_files %s: %w", path, err)
		}
		if utf8.Valid(decoded) && !bytes.ContainsRune(decoded, 0) {
			f.inline = string(decoded)
		} else {
			f.source = dataurl.EncodeBytes(decoded)
			f.useSource = true
			f.lineComment = "binary content emitted as a data: URL"
		}

		mode, hasMode, err := decodePermissions(m["permissions"])
		if err != nil {
			return fmt.Errorf("write_files %s: %w", path, err)
		}
		f.mode, f.hasMode = mode, hasMode

		doc.files = append(doc.files, f)
	}
	return nil
}


func decodeContent(content, encoding string) ([]byte, error) {
	raw := []byte(content)
	hasB64, hasGz := false, false
	for _, tok := range strings.Split(strings.ToLower(strings.TrimSpace(encoding)), "+") {
		switch strings.TrimSpace(tok) {
		case "", "text/plain":
		case "b64", "base64":
			hasB64 = true
		case "gz", "gzip":
			hasGz = true
		default:
			return nil, fmt.Errorf("unknown content encoding %q", tok)
		}
	}
	if hasB64 {
		dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content))
		if err != nil {
			return nil, fmt.Errorf("base64 decode: %w", err)
		}
		raw = dec
	}
	if hasGz {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		dec, err := io.ReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("gzip decode: %w", err)
		}
		raw = dec
	}
	return raw, nil
}

// decodePermissions mirrors cloud-init: an int is already decimal (YAML made 0644 into 420), a string is octal, missing is 0644.
func decodePermissions(v any) (mode int, has bool, err error) {
	switch p := v.(type) {
	case nil:
		return 0o644, true, nil
	case int:
		return p, true, nil
	case int64:
		return int(p), true, nil
	case float64:
		return int(p), true, nil
	case string:
		p = strings.TrimSpace(p)
		if p == "" {
			return 0o644, true, nil
		}
		n, err := strconv.ParseInt(p, 8, 32)
		if err != nil {
			return 0, false, fmt.Errorf("invalid octal permissions %q", p)
		}
		return int(n), true, nil
	default:
		return 0, false, fmt.Errorf("permissions must be an int or octal string, got %T", v)
	}
}

// toString coerces a scalar YAML value to a string; maps, lists and nil are not scalars and return false.
func toString(v any) (string, bool) {
	switch s := v.(type) {
	case nil:
		return "", false
	case string:
		return s, true
	case bool:
		return strconv.FormatBool(s), true
	case int:
		return strconv.Itoa(s), true
	case int64:
		return strconv.FormatInt(s, 10), true
	case float64:
		return strconv.FormatFloat(s, 'g', -1, 64), true
	default:
		return "", false
	}
}

// toStringList coerces a scalar or list into a slice of strings. Nil yields nil.
func toStringList(v any) []string {
	switch l := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(l))
		for _, item := range l {
			if s, ok := toString(item); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		if s, ok := toString(v); ok {
			return []string{s}
		}
		return nil
	}
}
