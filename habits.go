package main

import (
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type habitScreen struct {
	store      *store
	updates    chan<- any
	habits     []habit
	list       widget.List
	applyToday widget.Clickable
	done       widget.Clickable
	newItem    widget.Editor
	errors     errorList
	invalidate func()
}

func (hs *habitScreen) layout(gtx C, th *material.Theme) D {
	if hs.applyToday.Clicked() {
		go func() {
			hs.updates <- applyHabitsToToday{hs.habits}
		}()
	}
	if hs.done.Clicked() {
		go func() {
			hs.updates <- closeHabitScreen{}
		}()
	}
	layInfoPoint := func(gtx C, txt string) D {
		lbl := material.Body1(th, txt)
		return layout.Inset{Top: 12, Right: 20, Bottom: 12, Left: 20}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return iconInfo.Layout(gtx, th.ContrastBg)
				}),
				layout.Rigid(layout.Spacer{Width: 10}.Layout),
				layout.Rigid(lbl.Layout),
			)
		})
	}
	widgets := []layout.Widget{
		// Header.
		func(gtx C) D {
			heading := func(gtx C) D {
				lbl := material.H4(th, "Daily Habits")
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Min.X = 32
						return iconCheckCircle.Layout(gtx, th.Fg)
					}),
					layout.Rigid(layout.Spacer{Width: 12}.Layout),
					layout.Rigid(lbl.Layout),
				)
			}
			applyToday := material.Button(th, &hs.applyToday, "Apply to Today")
			done := material.Button(th, &hs.done, "Done")
			return layout.Inset{Top: 25, Right: 15, Bottom: 20, Left: 15}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, heading),
					layout.Rigid(applyToday.Layout),
					layout.Rigid(layout.Spacer{Width: 10}.Layout),
					layout.Rigid(done.Layout),
				)
			})
		},
		// Description about what daily habits are.
		func(gtx C) D {
			return layInfoPoint(gtx, "Your daily habits are things that you're striving to do every day.")
		},
		func(gtx C) D {
			return layInfoPoint(gtx, "This is where you create and edit the list that will be presented each day.")
		},
		// Items.
		func(gtx C) D {
			rows := make([]layout.FlexChild, len(hs.habits))
			for i := range hs.habits {
				item := &hs.habits[i]
				rows[i] = layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: 5}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return iconUnchecked.Layout(gtx, th.Fg)
							}),
							layout.Rigid(layout.Spacer{Width: 5}.Layout),
							layout.Rigid(material.Body1(th, item.Content).Layout),
						)
					})
				})
			}
			return layout.Inset{Top: 24, Right: 20, Bottom: 12, Left: 55}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
			})
		},
		// New item input.
		func(gtx C) D {
			for _, e := range hs.newItem.Events() {
				if e, ok := e.(widget.SubmitEvent); ok {
					hs.habits = append(hs.habits, habit{
						ID:        len(hs.habits) + 1,
						CreatedAt: time.Now(),
						Content:   e.Text,
					})
					go func() {
						if err := hs.store.putHabits(hs.habits); err != nil {
							hs.errors.add("saving habits", err)
							hs.invalidate()
						}
					}()
					hs.newItem.SetText("")
					op.InvalidateOp{}.Add(gtx.Ops)
				}
			}
			return layout.Inset{Top: 12, Right: 20, Bottom: 20, Left: 60}.Layout(gtx, func(gtx C) D {
				return editor{th, &hs.newItem, "Add new habit..."}.layout(gtx)
			})
		},
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return material.List(th, &hs.list).Layout(gtx, len(widgets), func(gtx C, i int) D {
				return widgets[i](gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return hs.errors.layout(gtx, th)
		}),
	)
}

type applyHabitsToToday struct {
	habits []habit
}

type closeHabitScreen struct{}
