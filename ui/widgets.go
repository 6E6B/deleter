package ui

import (
	"image/color"

	g "github.com/AllenDang/giu"
)

func errorLabel(msg string) g.Widget {
	return g.Style().SetColor(g.StyleColorText, color.RGBA{255, 80, 80, 255}).To(
		g.Label(msg).Wrapped(true),
	)
}
