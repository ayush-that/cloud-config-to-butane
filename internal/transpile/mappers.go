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

func mapUsers(v any, doc *document) error {
	items, ok := v.([]any)
	if !ok {
		return fmt.Errorf("users must be a list, got %T", v)
	}
	for i, raw := range items {
		switch u := raw.(type) {
		case string:
			if u == "default" {
				continue // the distro default user has no Ignition equivalent
			}
			doc.users = append(doc.users, user{name: u, headComment: "from users"})
		case map[string]any:
			name, _ := toString(u["name"])
			if name == "" {
				return fmt.Errorf("users[%d] is missing a name", i)
			}
			usr := user{name: name, headComment: "from users"}
			usr.groups = toStringList(u["groups"])
			usr.sshKeys = toStringList(firstPresent(u, "ssh_authorized_keys", "ssh-authorized-keys"))
			usr.shell, _ = toString(u["shell"])
			usr.primaryGroup, _ = toString(u["primary_group"])
			usr.passwordHash, _ = toString(firstPresent(u, "passwd", "hashed_passwd"))
			if sudo := u["sudo"]; sudo != nil && sudo != false {
				usr.groups = append(usr.groups, "sudo")
				usr.lineComment = "sudo mapped to 'sudo' group; exact sudoers rule not represented"
			}
			usr.groups = dedupe(usr.groups) // Ignition rejects duplicate group entries
			doc.users = append(doc.users, usr)
		default:
			return fmt.Errorf("users[%d] must be a string or mapping, got %T", i, raw)
		}
	}
	return nil
}

// mapGroups turns groups into passwd.groups entries; Ignition groups carry no membership, so members are dropped with a flag.
func mapGroups(v any, doc *document) error {
	add := func(name string, hasMembers bool) {
		g := group{name: name, headComment: "from groups"}
		if hasMembers {
			g.lineComment = "group members not representable in Ignition; set them via user.groups"
		}
		doc.groups = append(doc.groups, g)
	}
	switch g := v.(type) {
	case []any:
		for i, raw := range g {
			switch item := raw.(type) {
			case string:
				add(item, false)
			case map[string]any:
				for name, members := range item {
					add(name, len(toStringList(members)) > 0)
				}
			default:
				return fmt.Errorf("groups[%d] must be a string or mapping, got %T", i, raw)
			}
		}
	case map[string]any:
		for name, members := range g {
			add(name, len(toStringList(members)) > 0)
		}
	default:
		return fmt.Errorf("groups must be a list or mapping, got %T", v)
	}
	return nil
}

// mapCACerts writes each trusted cert to /etc/ssl/certs and adds a oneshot unit running update-ca-certificates.
func mapCACerts(v any, doc *document) error {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("ca_certs must be a mapping, got %T", v)
	}
	certs := toStringList(firstPresent(m, "trusted", "trusted_ca"))
	if len(certs) == 0 {
		return nil
	}
	for i, cert := range certs {
		cert = strings.TrimSpace(cert) + "\n"
		doc.files = append(doc.files, file{
			path:        fmt.Sprintf("/etc/ssl/certs/cloud-init-ca-cert-%d.pem", i+1),
			mode:        0o644,
			hasMode:     true,
			inline:      cert,
			headComment: "from ca_certs",
		})
	}
	doc.units = append(doc.units, unit{
		name:        "cc2butane-update-ca-certificates.service",
		enabled:     true,
		headComment: "from ca_certs",
		contents: joinUnit(
			"[Unit]",
			"Description=Update CA certificates (cc2butane, from cloud-config ca_certs)",
			"After=network-online.target",
			"",
			"[Service]",
			"Type=oneshot",
			"RemainAfterExit=yes",
			"ExecStart=/usr/sbin/update-ca-certificates",
			"",
			"[Install]",
			"WantedBy=multi-user.target",
		),
	})
	return nil
}

// mapCommands turns runcmd/bootcmd into a shell script plus a guarded oneshot unit; the script deletes the guard on success so it runs once across reboots.
func mapCommands(key string, v any, doc *document, opts Options) error {
	cmds, ok := v.([]any)
	if !ok {
		return fmt.Errorf("%s must be a list, got %T", key, v)
	}
	if len(cmds) == 0 {
		return nil
	}
	if opts.Strict {
		return &UnsupportedError{Key: key, Detail: "arbitrary commands are disabled under --strict"}
	}

	body, err := shellifyBody(cmds)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	base := "/etc/cc2butane/" + key
	script := base + ".sh"
	guard := base + ".guard"

	doc.files = append(doc.files,
		file{
			path:        script,
			mode:        0o755,
			hasMode:     true,
			inline:      "#!/bin/sh\nset -e\n" + body + "rm -f " + guard + "\n",
			headComment: "from " + key,
		},
		file{
			path:        guard,
			mode:        0o644,
			hasMode:     true,
			inline:      "# cc2butane " + key + " guard; the unit runs while this exists\n",
			headComment: "from " + key,
		},
	)

	u := unit{
		name:        "cc2butane-" + key + ".service",
		enabled:     true,
		headComment: "from " + key,
		contents: joinUnit(
			"[Unit]",
			"Description=cc2butane "+key+" (from cloud-config "+key+")",
			"ConditionPathExists="+guard,
			"After=network-online.target",
			"Wants=network-online.target",
			"",
			"[Service]",
			"Type=oneshot",
			"RemainAfterExit=yes",
			"ExecStart="+script,
			"",
			"[Install]",
			"WantedBy=multi-user.target",
		),
	}
	if key == "bootcmd" {
		u.lineComment = "bootcmd runs every boot in cloud-init; Ignition provisions once, so this runs once"
	}
	doc.units = append(doc.units, u)
	return nil
}

// shellifyBody reimplements cloud-init's shellify: a list item becomes single-quoted argv, a string item is emitted raw.
func shellifyBody(cmds []any) (string, error) {
	var b strings.Builder
	for i, c := range cmds {
		switch v := c.(type) {
		case string:
			b.WriteString(v)
			b.WriteByte('\n')
		case []any:
			parts := make([]string, len(v))
			for j, arg := range v {
				s, ok := toString(arg)
				if !ok {
					return "", fmt.Errorf("command %d argument %d is not a scalar", i, j)
				}
				parts[j] = "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
			}
			b.WriteString(strings.Join(parts, " "))
			b.WriteByte('\n')
		default:
			return "", fmt.Errorf("command %d must be a string or a list, got %T", i, c)
		}
	}
	return b.String(), nil
}

func joinUnit(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
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

// dedupe returns the input with later duplicates removed, preserving order.
func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := in[:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func firstPresent(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}
