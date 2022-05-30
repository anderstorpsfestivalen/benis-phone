package functions

type Action struct {
	Num  int
	Type string `toml:"t"`
	Dst  string
	Wait bool

	// actionables

	File       File       `toml:"file"`
	RandomFile RandomFile `toml:"randomfile"`
	Service    Service    `toml:"srv"`
}
