package sip

import (
	"bytes"
	"testing"

	"github.com/emiago/diago/media"
)

func TestRTPDTMFReaderAcceptsMinimumDurationEndPacket(t *testing.T) {
	packetReader := &media.RTPPacketReader{}
	packetReader.PacketHeader.PayloadType = media.CodecTelephoneEvent8000.PayloadType
	packetReader.PacketHeader.Timestamp = 1234

	payload := media.DTMFEncode(media.DTMFEvent{
		Event:      5,
		EndOfEvent: true,
		Duration:   320, // 40 ms at 8 kHz
	})
	reader := newRTPDTMFReader(media.CodecTelephoneEvent8000, packetReader, bytes.NewReader(payload))
	buf := make([]byte, media.RTPBufSize)
	if _, err := reader.Read(buf); err != nil {
		t.Fatalf("read DTMF: %v", err)
	}

	digit, ok := reader.ReadDTMF()
	if !ok || digit != '5' {
		t.Fatalf("got digit %q, ok=%v; want 5, true", digit, ok)
	}
}

func TestRTPDTMFReaderSuppressesRedundantEndPackets(t *testing.T) {
	reader := &rtpDTMFReader{codec: media.CodecTelephoneEvent8000}
	event := media.DTMFEvent{Event: 1, EndOfEvent: true, Duration: 800}

	reader.processEvent(1000, event)
	if digit, ok := reader.ReadDTMF(); !ok || digit != '1' {
		t.Fatalf("first end packet: got digit %q, ok=%v; want 1, true", digit, ok)
	}
	reader.processEvent(1000, event)
	if digit, ok := reader.ReadDTMF(); ok {
		t.Fatalf("redundant end packet emitted digit %q", digit)
	}

	reader.processEvent(2000, event)
	if digit, ok := reader.ReadDTMF(); !ok || digit != '1' {
		t.Fatalf("new event: got digit %q, ok=%v; want 1, true", digit, ok)
	}
}

func TestRTPDTMFReaderRejectsTooShortAndUnsupportedEvents(t *testing.T) {
	reader := &rtpDTMFReader{codec: media.CodecTelephoneEvent8000}

	reader.processEvent(1000, media.DTMFEvent{Event: 2, EndOfEvent: true, Duration: 319})
	if digit, ok := reader.ReadDTMF(); ok {
		t.Fatalf("short event emitted digit %q", digit)
	}

	reader.processEvent(2000, media.DTMFEvent{Event: 16, EndOfEvent: true, Duration: 800})
	if digit, ok := reader.ReadDTMF(); ok {
		t.Fatalf("unsupported event emitted digit %q", digit)
	}
}

func TestRTPDTMFReaderIgnoresAudioPayload(t *testing.T) {
	packetReader := &media.RTPPacketReader{}
	packetReader.PacketHeader.PayloadType = media.CodecAudioUlaw.PayloadType
	reader := newRTPDTMFReader(
		media.CodecTelephoneEvent8000,
		packetReader,
		bytes.NewReader([]byte{0, 0x80, 0x03, 0x20}),
	)
	buf := make([]byte, media.RTPBufSize)
	if _, err := reader.Read(buf); err != nil {
		t.Fatalf("read audio: %v", err)
	}
	if reader.LastReadWasDTMF() {
		t.Fatal("audio payload classified as DTMF")
	}
	if digit, ok := reader.ReadDTMF(); ok {
		t.Fatalf("audio payload emitted digit %q", digit)
	}
}
