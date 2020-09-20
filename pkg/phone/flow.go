package phone

type FlowPhone interface {
	Init() error
	Close()
	State() bool
	GetKeyChannel() chan string
	GetHookChannel() chan bool
}
