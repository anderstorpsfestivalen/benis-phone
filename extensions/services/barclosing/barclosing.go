package barclosing

import (
	"fmt"
	"time"
)

func ClosingTime() string {
	now := time.Now()

	add := 1

	if now.Hour() < 3 {
		add = 0
	}

	// Set closing time
	closing := time.Date(
		now.Year(), now.Month(), now.Day()+add, 01, 00, 00, 000000000, time.UTC)

	//Set location
	loc, _ := time.LoadLocation("Europe/Stockholm")

	newtime := closing.In(loc)

	//fmt.Printf("closing: %v\n", newtime)

	diff := newtime.Sub(now).Round(time.Minute)
	//diff := now.Sub(newtime).Round(time.Minute)

	message := fmtDuration(diff)
	return message

}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	return fmt.Sprintf("Baren stänger om, %d, timmar, och, %d, minuter", h, m)
}

type BarClosing struct {
}

func (f *BarClosing) Get(string) (string, error) {
	return ClosingTime(), nil
}
