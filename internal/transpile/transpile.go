package transpile

func Transpile(input []byte, opts Options) ([]byte, error) {
	cfg, err := parse(input)
	if err != nil {
		return nil, err
	}
	doc := &document{}
	if v, ok := cfg["write_files"]; ok {
		if err := mapWriteFiles(v, doc); err != nil {
			return nil, err
		}
	}
	if v, ok := cfg["users"]; ok {
		if err := mapUsers(v, doc); err != nil {
			return nil, err
		}
	}
	if v, ok := cfg["groups"]; ok {
		if err := mapGroups(v, doc); err != nil {
			return nil, err
		}
	}
	return emit(doc)
}
