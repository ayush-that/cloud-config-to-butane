package transpile

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// emit renders the IR as a Butane document with a fixed section order (variant, version, passwd, storage, systemd).
func emit(d *document) ([]byte, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	appendKV(root, "variant", str("flatcar"))
	appendKV(root, "version", str("1.1.0"))

	if len(d.groups) > 0 || len(d.users) > 0 {
		passwd := &yaml.Node{Kind: yaml.MappingNode}
		if len(d.groups) > 0 {
			appendKV(passwd, "groups", nodeSeq(d.groups, groupNode))
		}
		if len(d.users) > 0 {
			appendKV(passwd, "users", nodeSeq(d.users, userNode))
		}
		appendKV(root, "passwd", passwd)
	}
	if len(d.files) > 0 {
		storage := &yaml.Node{Kind: yaml.MappingNode}
		appendKV(storage, "files", nodeSeq(d.files, fileNode))
		appendKV(root, "storage", storage)
	}
	if len(d.units) > 0 {
		systemd := &yaml.Node{Kind: yaml.MappingNode}
		appendKV(systemd, "units", nodeSeq(d.units, unitNode))
		appendKV(root, "systemd", systemd)
	}

	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encoding butane yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("closing yaml encoder: %w", err)
	}
	return buf.Bytes(), nil
}

func fileNode(f file) *yaml.Node {
	content := []*yaml.Node{str("path"), str(f.path)}
	if f.hasMode {
		content = append(content, str("mode"), modeNode(f.mode))
	}
	var value *yaml.Node
	if f.useSource {
		value = str(f.source)
		value.LineComment = f.lineComment
		content = append(content, str("contents"), mapping(str("source"), value))
	} else {
		value = blockStr(f.inline)
		if f.lineComment != "" {
			value.LineComment = f.lineComment
		}
		content = append(content, str("contents"), mapping(str("inline"), value))
	}
	node := mapping(content...)
	node.HeadComment = f.headComment
	return node
}

func userNode(u user) *yaml.Node {
	name := str(u.name)
	name.LineComment = u.lineComment
	content := []*yaml.Node{str("name"), name}
	if u.passwordHash != "" {
		content = append(content, str("password_hash"), str(u.passwordHash))
	}
	if u.primaryGroup != "" {
		content = append(content, str("primary_group"), str(u.primaryGroup))
	}
	if len(u.groups) > 0 {
		content = append(content, str("groups"), strSeq(u.groups))
	}
	if u.shell != "" {
		content = append(content, str("shell"), str(u.shell))
	}
	if len(u.sshKeys) > 0 {
		content = append(content, str("ssh_authorized_keys"), strSeq(u.sshKeys))
	}
	node := mapping(content...)
	node.HeadComment = u.headComment
	return node
}

func groupNode(g group) *yaml.Node {
	name := str(g.name)
	name.LineComment = g.lineComment
	node := mapping(str("name"), name)
	node.HeadComment = g.headComment
	return node
}

func unitNode(u unit) *yaml.Node {
	name := str(u.name)
	name.LineComment = u.lineComment
	node := mapping(
		str("name"), name,
		str("enabled"), boolNode(u.enabled),
		str("contents"), blockStr(u.contents),
	)
	node.HeadComment = u.headComment
	return node
}

func str(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

// blockStr emits multiline strings as literal blocks for readable output.
func blockStr(v string) *yaml.Node {
	n := str(v)
	if strings.Contains(v, "\n") {
		n.Style = yaml.LiteralStyle
	}
	return n
}

// modeNode emits a file mode as an octal literal so yaml.v3 and Butane parse it back as the correct integer.
func modeNode(mode int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprintf("%#o", mode)}
}

func boolNode(b bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(b)}
}

func mapping(kv ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Content: kv}
}

func strSeq(items []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, item := range items {
		seq.Content = append(seq.Content, str(item))
	}
	return seq
}

func nodeSeq[T any](items []T, build func(T) *yaml.Node) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, item := range items {
		seq.Content = append(seq.Content, build(item))
	}
	return seq
}

func appendKV(m *yaml.Node, key string, value *yaml.Node) {
	m.Content = append(m.Content, str(key), value)
}
