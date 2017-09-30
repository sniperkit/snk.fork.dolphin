package types

// Template a unparsed template file
type Template struct {
	// Name  path where to store the parsed template
	Name string
	// Data tempalte  content
	Data []byte
}

// Charts is the config of an image
type Charts struct {
	Name        Name
	Version     string
	keyWords    []string
	Description string
	Owner       []string
	Values      map[string]string
	Templates   []Template
}

// Engine a render engine
type Engine interface {
	Render(Charts, map[string]string) (map[string]string, error)
}
