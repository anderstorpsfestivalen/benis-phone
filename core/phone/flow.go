// Package phone defines the FlowPhone interface — the contract a per-call
// keypad/hook source presents to the controller. The only production
// implementation today lives in core/sip (SIPPhone); historically there were
// also a GPIO and virtual-keyboard variant for local hardware testing.
package phone

type FlowPhone interface {
	Init() error
	Close()
	State() bool
	GetKeyChannel() chan string
	GetHookChannel() chan bool
}
