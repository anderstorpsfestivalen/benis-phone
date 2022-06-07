// package ui

// import (
// 	"fmt"
// 	"os"
// 	"sync"

// 	"github.com/gdamore/tcell/v2"
// 	"github.com/rivo/tview"
// 	"github.com/sirupsen/logrus"
// )

// type UI struct {
// 	log *logrus.Logger
// 	app *tview.Application
// }

// func Create(logger *logrus.Logger) *UI {
// 	app := tview.NewApplication()

// 	ui := UI{
// 		log: logger,
// 		app: app,
// 	}

// 	app.SetInputCapture(ui.capture)

// 	return &ui
// }

// func (u *UI) Run(wg *sync.WaitGroup) {
// 	newPrimitive := func(text string) tview.Primitive {
// 		return tview.NewTextView().
// 			SetTextAlign(tview.AlignCenter).
// 			SetText(text)
// 	}
// 	state := newPrimitive("State")
// 	input := newPrimitive("Input")
// 	log := newPrimitive("Log")

// 	grid := tview.NewGrid().
// 		SetRows(9).
// 		SetColumns(6).
// 		SetBorders(true).
// 		AddItem(state, 0, 0, 3, 4, 0, 100, false).
// 		AddItem(input, 0, 4, 3, 2, 0, 100, false).
// 		AddItem(log, 3, 0, 6, 6, 0, 0, false)

// 	if err := u.app.SetRoot(grid, true).Run(); err != nil {
// 		u.log.Fatal("nope")
// 	}
// }

// func (u *UI) capture(ev *tcell.EventKey) *tcell.EventKey {
// 	if ev.Key() == tcell.KeyEscape {
// 		u.app.Stop()
// 		os.Exit(0)
// 	}

// 	fmt.Println(ev.Name())

// 	return ev
// }

// func (u *UI) createLog() tview.Primitive {
// 	l := tview.NewList()

// 	l

// 	return l
// }
