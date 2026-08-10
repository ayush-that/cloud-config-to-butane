package transpile

import "fmt"

func (o Options) warnf(format string, a ...any) {
	if o.Warn != nil {
		fmt.Fprintf(o.Warn, format, a...)
	}
}

// Transpile converts a cloud-init #cloud-config document into a Flatcar Butane config.
func Transpile(input []byte, opts Options) ([]byte, error) {
	cfg, err := parse(input)
	if err != nil {
		return nil, err
	}
	doc := &document{}

	for _, key := range []string{"packages", "package_update", "package_upgrade"} {
		if v, ok := cfg[key]; ok && wantsPackages(v) {
			if opts.WarnUnsupported {
				opts.warnf("warning: dropping unsupported key %q (immutable OS has no package manager at provision time, use systemd-sysext)\n", key)
				continue
			}
			return nil, &UnsupportedError{Key: key, Detail: "immutable OS has no package manager at provision time, use systemd-sysext"}
		}
	}

	steps := []struct {
		key string
		fn  func(any, *document) error
	}{
		{"write_files", mapWriteFiles},
		{"users", mapUsers},
		{"groups", mapGroups},
		{"ca_certs", mapCACerts},
		{"runcmd", func(v any, d *document) error { return mapCommands("runcmd", v, d, opts) }},
		{"bootcmd", func(v any, d *document) error { return mapCommands("bootcmd", v, d, opts) }},
	}
	for _, s := range steps {
		if v, ok := cfg[s.key]; ok {
			if err := s.fn(v, doc); err != nil {
				return nil, err
			}
		}
	}

	return emit(doc)
}

// wantsPackages reports whether a packages key actually asks for work (a non-empty list or a true boolean).
func wantsPackages(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case []any:
		return len(t) > 0
	case nil:
		return false
	default:
		return true
	}
}
