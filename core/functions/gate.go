package functions

type Gate struct {
	Destination  string `toml:"dst"`
	Accept       string `toml:"accept"`
	Prompt       string `toml:"prompt"`
	Deny         string `toml:"deny"`
	DenyTemplate string `toml:"deny_tmpl"`
}
