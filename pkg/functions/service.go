package functions

type Service struct {
	Destination string            `toml:"dst"`
	Template    string            `toml:"tmpl"`
	Arguments   map[string]string `toml:"args"`
	TTS         TTS               `toml:"tts"`
}
