package phone

type FlowPhone interface {
	Init()
	Close()
	State() bool
	GetKeyChannel() chan string
	GetHookChannel() chan bool
}
