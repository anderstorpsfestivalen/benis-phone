package audio

// ConvertF64StereoToI16LE mixes stereo float64 samples to mono, clamps to
// [-1, 1], converts to int16, and writes them as little-endian bytes into out.
// Returns the number of bytes written (always 2 * len(in), assuming out is
// large enough).
func ConvertF64StereoToI16LE(in [][2]float64, out []byte) int {
	n := 0
	for i := range in {
		mono := (in[i][0] + in[i][1]) / 2.0
		if mono > 1.0 {
			mono = 1.0
		} else if mono < -1.0 {
			mono = -1.0
		}
		s := int16(mono * 32767)
		out[n] = byte(s)
		out[n+1] = byte(s >> 8)
		n += 2
	}
	return n
}

// ConvertF64StereoToI16Mono mixes and converts into an int16 slice. Returns
// number of samples written.
func ConvertF64StereoToI16Mono(in [][2]float64, out []int16) int {
	for i := range in {
		mono := (in[i][0] + in[i][1]) / 2.0
		if mono > 1.0 {
			mono = 1.0
		} else if mono < -1.0 {
			mono = -1.0
		}
		out[i] = int16(mono * 32767)
	}
	return len(in)
}
