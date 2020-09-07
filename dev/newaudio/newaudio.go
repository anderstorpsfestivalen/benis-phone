package main

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/audio"
	"gitlab.com/anderstorpsfestivalen/benis-phone/polly"
	"gitlab.com/anderstorpsfestivalen/benis-phone/secrets"
)

func main() {
	credentials := secrets.LoadSecrets()

	polly := polly.New(credentials.Polly.Key, credentials.Polly.Secret, "files/")

	ad := audio.New(44100)
	err := ad.Init()
	if err != nil {
		panic(err)
	}

	/// EXAMPLE FROM BYTESLICE STREAM

	data, err := polly.TTS("E-TYPE ELLER? GJORT I SVERIGE FOREVER", "Astrid")
	if err != nil {
		panic("EEEEEEEEEEETYPE ELLER")
	}

	err = ad.PlayMP3FromStream(data)

	///EXAMPLE PLAYING FROM FILE

	fmt.Println("PLAYING ETYPE")

	err = ad.PlayFromFile("files/etype.mp3")
	if err != nil {
		fmt.Println(err)
	}

}
