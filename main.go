package main

import (
	"flag"
	"image"
	"image/color"
	"log"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/steverusso/gio-fonts/vegur"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type habit struct {
	ID          int       `json:"id"`
	CreatedAt   time.Time `json:"created"`
	CompletedAt time.Time `json:"compl,omitempty"`
	DeletedAt   time.Time `json:"del,omitempty"`
	Content     string    `json:"cont,omitempty"`
}

func (h *habit) isDone() bool {
	return !h.CompletedAt.IsZero()
}

func (h *habit) isDeleted() bool {
	return !h.DeletedAt.IsZero()
}

type dailySummary struct {
	NumCompl int     `json:"n"`
	PctCompl float32 `json:"p"`
}

func newSummaryOfList(list []habit) dailySummary {
	numCompl := 0
	for _, h := range list {
		if h.isDone() {
			numCompl++
		}
	}
	return dailySummary{
		NumCompl: numCompl,
		PctCompl: float32(numCompl) / float32(len(list)),
	}
}

type App struct {
	store     *store
	splashErr splashErr
	home      homeScreen
	habits    *habitScreen
}

func (a *App) handleKeyEvent(ke key.Event) {
	if a.habits != nil {
		switch ke.Name {
		case "/":
			a.habits.newItem.Focus()
		case key.NameEscape:
			a.habits = nil
		}
		return
	}
}

func (a *App) layout(gtx C, th *material.Theme) D {
	paint.Fill(gtx.Ops, th.Bg)
	if a.store == nil {
		if a.splashErr != nil {
			return layout.Center.Layout(gtx, func(gtx C) D {
				return material.Body1(th, a.splashErr.Error()).Layout(gtx)
			})
		}
		return layout.Center.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.X = 300
			return material.H6(th, "Loading...").Layout(gtx)
		})
	}
	if a.habits != nil {
		return a.habits.layout(gtx, th)
	}
	return a.home.layout(gtx, th)
}

func (a *App) mergeHabitTemplateWithToday(u applyHabitsToToday) {
	fmtDate := time.Now().Format("060102")
	todaysHabits, err := a.store.getHabitsForDay(fmtDate)
	if err != nil {
		a.home.errors.add("reading today's habits", err)
		return
	}
	var resolved []habit
	for _, tmplHabit := range u.habits {
		var currentHabit *habit
		for i, c := range todaysHabits {
			if c.ID == tmplHabit.ID && !tmplHabit.isDeleted() {
				currentHabit = &todaysHabits[i]
				break
			}
		}
		if currentHabit != nil {
			resolved = append(resolved, *currentHabit)
		} else {
			resolved = append(resolved, tmplHabit)
		}
	}
	if err := a.store.putHabitsForDay(fmtDate, resolved); err != nil {
		a.home.errors.add("saving today's new habits", err)
		return
	}
	a.home.setSummary(fmtDate, newSummaryOfList(resolved))
	if a.home.record.fmtDate == fmtDate {
		checks := make([]widget.Bool, len(resolved))
		for i := range resolved {
			checks[i] = widget.Bool{Value: resolved[i].isDone()}
		}
		a.home.record.checks = checks
		a.home.record.habits = resolved
	}
}

type splashErr error

type splashHandOff struct {
	store     *store
	summaries map[string]dailySummary
	record    dailyRecordWidget
}

func initLoad(dbFile string, updates chan<- any) {
	store, err := openStore(dbFile)
	if err != nil {
		updates <- splashErr(err)
		return
	}
	summaries, err := store.getSummaries()
	if err != nil {
		updates <- splashErr(err)
		return
	}
	fmtDate := time.Now().Format("060102")
	todaysHabits, err := store.getHabitsForDay(fmtDate)
	if err != nil {
		updates <- splashErr(err)
		return
	}
	record, err := newDailyRecord(fmtDate, todaysHabits)
	if err != nil {
		updates <- splashErr(err)
		return
	}
	updates <- splashHandOff{
		store:     store,
		summaries: summaries,
		record:    record,
	}
}

func run(dbFile string, showFrameTimes bool) error {
	updates := make(chan any)

	go initLoad(dbFile, updates)

	win := app.NewWindow(
		app.Size(720, 720),
		app.Title("Todaily"),
	)
	win.Perform(system.ActionCenter)

	th := material.NewTheme(vegur.Collection())
	th.TextSize = 17
	th.Palette = material.Palette{
		Bg:         color.NRGBA{17, 21, 24, 255},
		Fg:         color.NRGBA{230, 230, 230, 255},
		ContrastFg: color.NRGBA{251, 251, 251, 255},
		ContrastBg: color.NRGBA{40, 170, 196, 255},
	}

	var a App
	var ops op.Ops
	for {
		select {
		case u := <-updates:
			switch u := u.(type) {
			case splashErr:
				a.splashErr = u
			case splashHandOff:
				a.store = u.store
				a.home = homeScreen{
					store:      a.store,
					updates:    updates,
					gridRows:   newDayGrid(u.summaries),
					habitList:  widget.List{List: layout.List{Axis: layout.Vertical}},
					record:     u.record,
					invalidate: win.Invalidate,
				}
			case openHabitScreen:
				a.habits = &habitScreen{
					store:      a.store,
					updates:    updates,
					list:       widget.List{List: layout.List{Axis: layout.Vertical}},
					newItem:    widget.Editor{SingleLine: true, Submit: true},
					habits:     u.items,
					invalidate: win.Invalidate,
				}
			case applyHabitsToToday:
				a.mergeHabitTemplateWithToday(u)
			case closeHabitScreen:
				a.habits = nil
			}
			win.Invalidate()
		case e := <-win.Events():
			switch e := e.(type) {
			case system.FrameEvent:
				start := time.Now()
				gtx := layout.NewContext(&ops, e)
				// Process any key events since the previous frame.
				for _, ke := range gtx.Events(win) {
					if ke, ok := ke.(key.Event); ok {
						a.handleKeyEvent(ke)
					}
				}
				// Gather key input on the entire window area.
				areaStack := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
				key.InputOp{Tag: win, Keys: "/|" + key.NameEscape}.Add(gtx.Ops)
				a.layout(gtx, th)
				areaStack.Pop()
				e.Frame(gtx.Ops)
				if showFrameTimes {
					log.Println(time.Since(start))
				}
			case system.DestroyEvent:
				return e.Err
			}
		}
	}
}

func main() {
	showFrameTimes := flag.Bool("print-frame-times", false, "Print out how long each frame takes.")
	flag.Parse()
	dbFile := flag.Arg(0)

	go func() {
		if err := run(dbFile, *showFrameTimes); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()

	app.Main()
}
