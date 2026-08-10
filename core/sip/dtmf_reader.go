package sip

import (
	"io"

	"github.com/emiago/diago/media"
	log "github.com/sirupsen/logrus"
)

// RFC 4733 cites 40 ms as the minimum recognizable DTMF signal duration.
// Event durations are expressed in units of the negotiated clock rate.
const minimumDTMFDurationMS = 40

// rtpDTMFReader inspects the RTP payload read by reader and extracts
// telephone-event digits. diago's RTPDtmfReader compares the final duration
// with the first packet's duration, which rejects otherwise valid 40-60 ms
// events. It also requires a non-final packet to arrive before the final one,
// even though RFC 4733 deliberately repeats end packets to tolerate loss.
type rtpDTMFReader struct {
	codec        media.Codec
	packetReader *media.RTPPacketReader
	reader       io.Reader

	digit    rune
	digitSet bool

	lastEventTimestamp uint32
	lastEvent          uint8
	lastEventSet       bool
	lastEventEmitted   bool
	lastReadWasDTMF    bool
}

func newRTPDTMFReader(codec media.Codec, packetReader *media.RTPPacketReader, reader io.Reader) *rtpDTMFReader {
	return &rtpDTMFReader{
		codec:        codec,
		packetReader: packetReader,
		reader:       reader,
	}
}

func (r *rtpDTMFReader) Read(buf []byte) (int, error) {
	n, err := r.reader.Read(buf)
	if err != nil {
		return n, err
	}

	r.lastReadWasDTMF = r.packetReader.PacketHeader.PayloadType == r.codec.PayloadType
	if !r.lastReadWasDTMF {
		return n, nil
	}

	event := media.DTMFEvent{}
	if err := media.DTMFDecode(buf[:n], &event); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"pt":    r.codec.PayloadType,
			"bytes": n,
		}).Debug("Invalid RTP DTMF packet")
		return n, nil
	}

	timestamp := r.packetReader.PacketHeader.Timestamp
	r.processEvent(timestamp, event)
	return n, nil
}

func (r *rtpDTMFReader) processEvent(timestamp uint32, event media.DTMFEvent) {
	if !r.lastEventSet || timestamp != r.lastEventTimestamp || event.Event != r.lastEvent {
		r.lastEventTimestamp = timestamp
		r.lastEvent = event.Event
		r.lastEventSet = true
		r.lastEventEmitted = false

		log.WithFields(log.Fields{
			"event":     event.Event,
			"pt":        r.codec.PayloadType,
			"timestamp": timestamp,
		}).Debug("RTP DTMF event started")
	}

	if !event.EndOfEvent || r.lastEventEmitted {
		return
	}

	durationMS := uint64(event.Duration) * 1000 / uint64(r.codec.SampleRate)
	if durationMS < minimumDTMFDurationMS {
		log.WithFields(log.Fields{
			"duration_ms": durationMS,
			"event":       event.Event,
		}).Debug("Ignoring RTP DTMF event shorter than 40 ms")
		return
	}
	if event.Event > 15 {
		log.WithField("event", event.Event).Debug("Ignoring unsupported RTP DTMF event")
		return
	}

	r.digit = media.DTMFToRune(event.Event)
	r.digitSet = true
	r.lastEventEmitted = true
}

func (r *rtpDTMFReader) ReadDTMF() (rune, bool) {
	digit, ok := r.digit, r.digitSet
	r.digitSet = false
	return digit, ok
}

func (r *rtpDTMFReader) LastReadWasDTMF() bool {
	return r.lastReadWasDTMF
}
