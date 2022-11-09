package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var (
	iconChecked     = mustIcon(icons.ToggleCheckBox)
	iconCheckCircle = mustIcon(icons.ActionCheckCircle)
	iconError       = mustIcon(icons.AlertError)
	iconEvent       = mustIcon(icons.ActionEvent)
	iconInfo        = mustIcon(icons.ActionInfo)
	iconUnchecked   = mustIcon(icons.ToggleCheckBoxOutlineBlank)
	iconWarning     = mustIcon(icons.AlertWarning)
)

// mustIcon returns a new `*widget.Icon` for the given byte slice. It panics on error.
func mustIcon(data []byte) *widget.Icon {
	ic, err := widget.NewIcon(data)
	if err != nil {
		panic(err)
	}
	return ic
}

type errorList struct {
	errors []errWidget
}

func (el *errorList) add(desc string, err error) {
	el.errors = append(el.errors, errWidget{desc: desc, err: err})
}

func (el *errorList) layout(gtx C, th *material.Theme) D {
	for i := len(el.errors) - 1; i >= 0; i-- {
		if el.errors[i].dismiss.Clicked() {
			el.errors = append(el.errors[:i], el.errors[i+1:]...)
		}
	}
	flexErrs := make([]layout.FlexChild, len(el.errors))
	for i := range el.errors {
		err := &el.errors[i]
		flexErrs[i] = layout.Rigid(func(gtx C) D {
			return err.layout(gtx, th)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, flexErrs...)
}

type errWidget struct {
	desc    string
	err     error
	dismiss widget.Clickable
}

func (e *errWidget) layout(gtx C, th *material.Theme) D {
	darkRed := color.NRGBA{132, 26, 5, 255}

	dismissBtn := material.Button(th, &e.dismiss, "Dismiss")
	dismissBtn.Background = darkRed
	dismissBtn.Inset = layout.Inset{Top: 5, Right: 10, Bottom: 5, Left: 10}

	layHeading := func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return iconError.Layout(gtx, darkRed)
			}),
			layout.Rigid(layout.Spacer{Width: 10}.Layout),
			layout.Flexed(1, material.Label(th, th.TextSize*20.0/18.0, "Error "+e.desc).Layout),
			layout.Rigid(dismissBtn.Layout),
		)
	}
	layBody := func(gtx C) D {
		return layout.Inset{Left: 15}.Layout(gtx, material.Body1(th, e.err.Error()).Layout)
	}
	layInner := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(layHeading),
			layout.Rigid(layout.Spacer{Height: 5}.Layout),
			layout.Rigid(layBody),
		)
	}
	// Get the dimensions of the whole error widget so we can know what
	// rectangle to fill for the background.
	m := op.Record(gtx.Ops)
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.UniformInset(10).Layout(gtx, layInner)
		}),
		layout.Rigid(rule{color: darkRed}.layout),
	)
	call := m.Stop()
	// Fill the background.
	rect := clip.Rect{Max: dims.Size}.Op()
	paint.FillShape(gtx.Ops, color.NRGBA{183, 93, 75, 255}, rect)
	call.Add(gtx.Ops)
	return dims
}

type rule struct {
	width int
	color color.NRGBA
	axis  layout.Axis
}

func (rl rule) layout(gtx C) D {
	if rl.width == 0 {
		rl.width = 1
	}
	size := image.Point{gtx.Constraints.Max.X, rl.width}
	if rl.axis == layout.Vertical {
		size = image.Point{rl.width, gtx.Constraints.Max.Y}
	}
	rect := clip.Rect{Max: size}.Op()
	paint.FillShape(gtx.Ops, rl.color, rect)
	return D{Size: size}
}

type editor struct {
	th    *material.Theme
	state *widget.Editor
	hint  string
}

func (e editor) layout(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.Editor(e.th, e.state, e.hint).Layout),
		layout.Rigid(rule{color: e.th.Fg}.layout),
	)
}
