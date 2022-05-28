package functions

type Action struct {
	Num  int
	Type string `toml:"t"`
	Dst  string
	Wait bool
}
