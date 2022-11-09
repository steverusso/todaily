package main

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	cellWidth  = 29
	cellHeight = 17
)

var monthAbbrevs = [12]string{
	"Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
}

type homeScreen struct {
	store      *store
	updates    chan<- any
	gridRows   []gridRow
	editHabits widget.Clickable
	habitList  widget.List
	record     dailyRecordWidget
	errors     errorList
	invalidate func()
}

func (hs *homeScreen) layout(gtx C, th *material.Theme) D {
	eventIcon := func(gtx C) D {
		gtx.Constraints.Min.X = 32
		gtx.Constraints.Max.X = 32
		return iconEvent.Layout(gtx, th.Fg)
	}
	lbl := material.H6(th, hs.record.prettyDate)
	header := func(gtx C) D {
		return layout.UniformInset(15).Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(eventIcon),
				layout.Rigid(layout.Spacer{Width: 12}.Layout),
				layout.Rigid(lbl.Layout),
			)
		})
	}
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return hs.laySidebar(gtx, th)
		}),
		layout.Rigid(rule{
			width: 1,
			color: th.Fg,
			axis:  layout.Vertical,
		}.layout),
		layout.Flexed(1, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(header),
				layout.Rigid(rule{width: 1, color: th.Fg}.layout),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Flexed(1, func(gtx C) D { return hs.layHabits(gtx, th) }),
				layout.Rigid(layout.Spacer{Height: 10}.Layout),
				layout.Rigid(func(gtx C) D {
					return hs.errors.layout(gtx, th)
				}),
			)
		}),
	)
}

func (hs *homeScreen) laySidebar(gtx C, th *material.Theme) D {
	if hs.editHabits.Clicked() {
		go func() {
			items, err := hs.store.getHabits()
			if err != nil {
				hs.errors.add("reading habits", err)
				hs.invalidate()
				return
			}
			hs.updates <- openHabitScreen{items}
		}()
	}
	// Determine which month abbreviation takes up the most horizontal space
	// so we can determine the width of the first flex column.
	var monthColWidth int
	for m := time.January; m <= time.December; m++ {
		macro := op.Record(gtx.Ops)
		dims := material.Label(th, 12, monthAbbrevs[m-1]).Layout(gtx)
		_ = macro.Stop()
		if w := dims.Size.X; w > monthColWidth {
			monthColWidth = w
		}
	}
	outerInset := layout.Inset{Top: 5, Right: 12, Bottom: 5, Left: 5}
	monthColInset := layout.Inset{Top: 2, Right: 5, Bottom: 2, Left: 2}
	totalWidth := monthColWidth + 7*(4+cellWidth) + int(monthColInset.Left) + int(monthColInset.Right) + int(outerInset.Left) + int(outerInset.Right)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return outerInset.Layout(gtx, func(gtx C) D {
				return hs.layDaySelection(gtx, th, monthColWidth, monthColInset)
			})
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = totalWidth
			return sidebarButton(gtx, th, &hs.editHabits, iconCheckCircle, "Manage Habits")
		}),
	)
}

func (hs *homeScreen) layDaySelection(gtx C, th *material.Theme, monthColWidth int, monthColInset layout.Inset) D {
	now := time.Now()
	flexRows := make([]layout.FlexChild, len(hs.gridRows)+1)
	// The first row in the navigation grid is for displaying the first letter
	// of each day of the week.
	flexRows[0] = layout.Rigid(func(gtx C) D {
		var letters [8]layout.FlexChild
		// There will never be a month abbreviation on the same row as the
		// weekday letters, so we just need to fill that space in here.
		letters[0] = layout.Rigid(func(gtx C) D {
			return monthColInset.Layout(gtx, func(gtx C) D {
				return D{Size: image.Pt(monthColWidth, cellHeight)}
			})
		})
		for i, c := range "SMTWTFS" {
			lbl := material.Label(th, 12, string(c))
			lbl.Alignment = text.Middle
			letters[i+1] = layout.Rigid(func(gtx C) D {
				return layout.UniformInset(2).Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = cellWidth // Set the min width so the label will center properly.
					dims := lbl.Layout(gtx)
					return D{Size: image.Point{X: cellWidth, Y: dims.Size.Y}}
				})
			})
		}
		return layout.Flex{}.Layout(gtx, letters[:]...)
	})
	for i := range hs.gridRows {
		r := &hs.gridRows[i]
		var rowOfCells [8]layout.FlexChild
		// The first slot in the row is reserved for showing the month's
		// abbreviation if this will be the first full row of the month.
		rowOfCells[0] = layout.Rigid(func(gtx C) D {
			return monthColInset.Layout(gtx, func(gtx C) D {
				if r.monthText != "" {
					dims := material.Label(th, 12, r.monthText).Layout(gtx)
					return D{Size: image.Pt(monthColWidth, dims.Size.Y)}
				}
				return D{Size: image.Pt(monthColWidth, cellHeight)}
			})
		})
		for j := range r.cells {
			cell := &r.cells[j]
			if cell.Clicked(gtx) {
				go hs.selectDay(cell.fmtDate)
			}
			layCell := func(gtx C) D {
				if cell.day.After(now) {
					return D{}
				}
				size := image.Pt(cellWidth, cellHeight)
				// Draw a thin border around the selected day's cell or if a cell is hovered.
				if cell.click.Hovered() || hs.record.fmtDate == cell.fmtDate {
					paint.FillShape(gtx.Ops, th.Fg, clip.Rect{
						Min: image.Pt(-1, -1),
						Max: image.Pt(cellWidth+1, cellHeight+1),
					}.Op())
					paint.FillShape(gtx.Ops, th.Bg, clip.Rect{Max: size}.Op())
				}
				clr := color.NRGBA{80, 80, 80, 132}
				dims := drawSquare(gtx, clr, cellWidth, cellHeight) // Cell background.
				if p := cell.summary.PctCompl; p > 0 {
					clr = th.ContrastBg
					clr.A = byte(float32(255) * p)
					drawSquare(gtx, clr, int(cellWidth*p), cellHeight) // Cell completion progress.
				}
				defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
				cell.click.Add(gtx.Ops)
				return dims
			}
			rowOfCells[j+1] = layout.Rigid(func(gtx C) D {
				return layout.UniformInset(2).Layout(gtx, layCell)
			})
		}
		flexRows[i+1] = layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, rowOfCells[:]...)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, flexRows...)
}

func sidebarButton(gtx C, th *material.Theme, click *widget.Clickable, ic *widget.Icon, txt string) D {
	layInner := func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return ic.Layout(gtx, th.Fg)
			}),
			layout.Rigid(layout.Spacer{Width: 8}.Layout),
			layout.Flexed(1, material.Label(th, unit.Sp(float32(th.TextSize)*1.1), txt).Layout),
		)
	}
	return material.Clickable(gtx, click, func(gtx C) D {
		return layout.UniformInset(6).Layout(gtx, layInner)
	})
}

func layItem(gtx C, th *material.Theme, check *widget.Bool, content string) D {
	lbl := material.Body1(th, content)
	box := func(gtx C) D {
		icon := iconUnchecked
		if check.Value {
			icon = iconChecked
		}
		return icon.Layout(gtx, th.ContrastBg)
	}
	return check.Layout(gtx, func(gtx C) D {
		return layout.Inset{Top: 5, Right: 20, Bottom: 5, Left: 20}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(box),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(lbl.Layout),
			)
		})
	})
}

func (hs *homeScreen) layHabits(gtx C, th *material.Theme) D {
	if len(hs.record.habits) == 0 {
		icon := iconWarning
		clr := color.NRGBA{234, 180, 4, 255}
		msg := material.Body1(th, "You didn't have any habits set on this day.").Layout
		if hs.record.fmtDate == time.Now().Format("060102") {
			icon = iconInfo
			clr = color.NRGBA{30, 180, 200, 255}
			msg = func(gtx C) D {
				one := material.Body1(th, "No habits created yet!")
				two := material.Body1(th, "Click 'Manage Habits' in the bottom of the sidebar.")
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(one.Layout),
					layout.Rigid(layout.Spacer{Height: 10}.Layout),
					layout.Rigid(two.Layout),
				)
			}
		}
		layIcon := func(gtx C) D {
			gtx.Constraints.Min.X = 32
			gtx.Constraints.Max.X = 32
			return icon.Layout(gtx, clr)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.UniformInset(15).Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(layIcon),
						layout.Rigid(layout.Spacer{Width: 10}.Layout),
						layout.Rigid(msg),
					)
				})
			}),
		)
	}
	return material.List(th, &hs.habitList).Layout(gtx, len(hs.record.habits), func(gtx C, i int) D {
		item := &hs.record.habits[i]
		check := &hs.record.checks[i]
		if check.Changed() {
			var t time.Time
			if check.Value {
				t = time.Now()
			}
			item.CompletedAt = t
			go hs.saveCurrentRecord()
			op.InvalidateOp{}.Add(gtx.Ops)
		}
		return layItem(gtx, th, check, item.Content)
	})
}

func (hs *homeScreen) selectDay(fmtDate string) {
	defer hs.invalidate()
	items, err := hs.store.getHabitsForDay(fmtDate)
	if err != nil {
		hs.errors.add("selecting day", err)
		return
	}
	hs.record, err = newDailyRecord(fmtDate, items)
	if err != nil {
		hs.errors.add("selecting day", err)
	}
}

func (hs *homeScreen) saveCurrentRecord() {
	defer hs.invalidate()
	newSummary := newSummaryOfList(hs.record.habits)
	hs.setSummary(hs.record.fmtDate, newSummary)
	if err := hs.store.putHabitsForDay(hs.record.fmtDate, hs.record.habits); err != nil {
		hs.errors.add("saving this day's habits", err)
	}
}

func (hs *homeScreen) setSummary(fmtDate string, summary dailySummary) {
	for i := range hs.gridRows {
		cells := &hs.gridRows[i].cells
		for j := range cells {
			if cells[j].fmtDate == fmtDate {
				cells[j].summary = summary
				return
			}
		}
	}
}

type gridRow struct {
	monthText string
	cells     [7]gridCell
}

type gridCell struct {
	day     time.Time
	fmtDate string
	click   gesture.Click
	summary dailySummary
}

func (c *gridCell) Clicked(gtx C) bool {
	for _, e := range c.click.Events(gtx) {
		if e.Type == gesture.TypeClick {
			return true
		}
	}
	return false
}

func newDayGrid(summaries map[string]dailySummary) []gridRow {
	// Start date is six months ago, rounded back to the nearest Sunday,
	// and the end date is today.
	now := time.Now()
	start := now.AddDate(0, -6, 0)
	start = start.AddDate(0, 0, 0-int(start.Weekday()))
	end := now

	numRows := int(end.Sub(start).Hours()/24)/7 + 2
	rows := make([]gridRow, 0, numRows)

	for day := start; day.Before(end.AddDate(0, 0, 1)); {
		var r gridRow
		if day.Day() <= 7 {
			r.monthText = monthAbbrevs[day.Month()-1]
		}
		for i := 0; i < 7; i++ {
			fmtDate := day.Format("060102")
			r.cells[i] = gridCell{
				day:     day,
				fmtDate: fmtDate,
				summary: summaries[fmtDate],
			}
			day = day.AddDate(0, 0, 1)
		}
		rows = append(rows, r)
	}
	return rows
}

func drawSquare(gtx C, clr color.NRGBA, w, h int) D {
	size := image.Pt(w, h)
	rect := clip.Rect{Max: size}.Op()
	paint.FillShape(gtx.Ops, clr, rect)
	return D{Size: size}
}

type dailyRecordWidget struct {
	fmtDate    string
	prettyDate string
	habits     []habit
	checks     []widget.Bool
}

func newDailyRecord(fmtDate string, habits []habit) (dailyRecordWidget, error) {
	checks := make([]widget.Bool, len(habits))
	for i := range habits {
		checks[i] = widget.Bool{Value: habits[i].isDone()}
	}
	t, err := time.ParseInLocation("060102", fmtDate, time.Now().Location())
	if err != nil {
		return dailyRecordWidget{}, fmt.Errorf("parsing fmtDate %q: %w", fmtDate, err)
	}
	return dailyRecordWidget{
		fmtDate:    fmtDate,
		prettyDate: t.Format("Jan 2, 2006"),
		habits:     habits,
		checks:     checks,
	}, nil
}

type openHabitScreen struct {
	items []habit
}
