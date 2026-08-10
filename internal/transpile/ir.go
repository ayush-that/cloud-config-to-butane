package transpile

// document is the IR between cloud-config and Butane; headComment renders above a block, lineComment inline.
type document struct {
	files  []file
	users  []user
	groups []group
	units  []unit
}

// Exactly one of inline or source is set.
type file struct {
	path    string
	mode    int // decimal file mode, e.g. 0644 == 420
	hasMode bool

	inline    string
	source    string
	useSource bool

	headComment string
	lineComment string
}

type user struct {
	name         string
	groups       []string
	sshKeys      []string
	shell        string
	primaryGroup string
	passwordHash string

	headComment string
	lineComment string
}

type group struct {
	name        string
	headComment string
	lineComment string
}

type unit struct {
	name        string
	enabled     bool
	contents    string
	headComment string
	lineComment string
}

func (d *document) empty() bool {
	return len(d.files) == 0 && len(d.users) == 0 && len(d.groups) == 0 && len(d.units) == 0
}
