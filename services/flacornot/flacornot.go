package flacornot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type APIResopnse struct {
	Spotify struct {
		Format string `json:"format"`
		State  string `json:"state"`
		Album  string `json:"album"`
		Artist string `json:"artist"`
		Song   string `json:"song"`
	} `json:"spotify"`
	Swinsian struct {
		Format string `json:"format"`
		State  string `json:"state"`
		Album  string `json:"album"`
		Artist string `json:"artist"`
		Song   string `json:"song"`
	} `json:"swinsian"`
}

type FlacOrNot struct {
}

func (f *FlacOrNot) Get(string) (string, error) {

	s := APIResopnse{}
	// temp for testing
	//res, err := http.Get("https://files.anderstorpsfestivalen.se/dump/playing.json")
	// ATP prod IP
	res, err := http.Get("http://45.154.31.62:8080")

	if err != nil {
		return "", fmt.Errorf("Could not craft request from API")
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(body, &s)

	if s.Swinsian.State == "" {
		return "", fmt.Errorf("No good parse smoke weed everyday 4Head")
	}

	message := ""

	if s.Swinsian.State == "playing" {
		message = "Currently, playing, " + s.Swinsian.Artist + ", " + s.Swinsian.Song + " , from, album, " + s.Swinsian.Album + ", song quality, is, " + s.Swinsian.Format
	} else if s.Spotify.State == "playing" {
		message = "Currently, playing, " + s.Spotify.Artist + ", " + s.Spotify.Song + " , from, album, " + s.Spotify.Album + ", song quality, is, " + s.Spotify.Format
	} else {
		message = "No song is currently playing."
	}

	return message, nil

}
