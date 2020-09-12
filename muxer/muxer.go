package muxer

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/phone"
)

type Muxer struct {
	KeyChannel  chan string
	HookChannel chan bool
	a           phone.FlowPhone
	b           phone.FlowPhone
}

func New(a phone.FlowPhone, b phone.FlowPhone) *Muxer {
	return &Muxer{
		KeyChannel:  make(chan string, 1),
		HookChannel: make(chan bool, 1),
		a:           a,
		b:           b,
	}
}

func (m *Muxer) Init() error {
	err := m.a.Init()
	if err != nil {
		return err
	}

	err = m.b.Init()
	if err != nil {
		return err
	}

	aHook := m.a.GetHookChannel()
	bHook := m.b.GetHookChannel()
	go func() {
		for {
			select {
			case msg1 := <-aHook:
				m.HookChannel <- msg1
			case msg2 := <-bHook:
				m.HookChannel <- msg2
			}
		}
	}()

	aKey := m.a.GetKeyChannel()
	bKey := m.b.GetKeyChannel()
	go func() {
		for {
			select {
			case msg1 := <-aKey:
				m.KeyChannel <- msg1
			case msg2 := <-bKey:
				m.KeyChannel <- msg2
			}
		}
	}()

	return nil
}
func (m *Muxer) Close() {

}
func (m *Muxer) State() bool {
	return true
}

func (m *Muxer) GetKeyChannel() chan string {
	return m.KeyChannel
}

func (m *Muxer) GetHookChannel() chan bool {
	return m.HookChannel
}
