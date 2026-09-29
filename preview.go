// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2ter

import (
	"image"
	"image/color"
	"math"

	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2riv"
)

var (
	landformColors = map[Landform]color.RGBA{
		FreshWater:        {0x1f, 0x78, 0xd1, 0xff},
		Flats:             {0xd8, 0xe8, 0xa8, 0xff},
		Plains:            {0xa8, 0xd0, 0x78, 0xff},
		RollingPlains:     {0x72, 0xae, 0x55, 0xff},
		Hills:             {0xc8, 0xa4, 0x68, 0xff},
		Mountains:         {0x8a, 0x66, 0x4a, 0xff},
		Plateaus:          {0xe0, 0x8a, 0x3a, 0xff},
		VolcanicHighlands: {0xb8, 0x4a, 0x2a, 0xff},
		Cliffs:            {0xc0, 0x10, 0x10, 0xff},
		Badlands:          {0x70, 0x20, 0x40, 0xff},
	}
	depthColors = map[Depth]color.RGBA{
		Shallow: {0x4f, 0x93, 0xc4, 0xff},
		Open:    {0x2b, 0x57, 0x8c, 0xff},
		Deep:    {0x14, 0x2c, 0x55, 0xff},
	}
	coastWaterColor = color.RGBA{0x8e, 0xc9, 0xe8, 0xff}
	inlandSeaColor  = color.RGBA{0x2a, 0xa0, 0x98, 0xff}
	impassableColor = color.RGBA{0x80, 0x30, 0x30, 0xff}
	volcanoColor    = color.RGBA{0xff, 0x00, 0xc8, 0xff}
	riverColor      = color.RGBA{0x10, 0x3a, 0xc8, 0xff}
)

// hexColor returns the preview color of a hex: salt water by depth, with
// coast and inland seas lighter; other hexes by landform; impassable land
// and volcanoes in their own colors.
func hexColor(h Hex) color.RGBA {
	c := landformColors[h.Landform]
	if h.Landform == SaltWater {
		c = depthColors[h.Depth]
	}
	for _, f := range h.Flags {
		switch {
		case f == InlandSea:
			c = inlandSeaColor
		case f == Coast && h.Landform == SaltWater:
			c = coastWaterColor
		case f == Impassable && h.Landform.IsLand():
			c = impassableColor
		case f == Volcano:
			c = volcanoColor
		}
	}
	return c
}

// RenderPreview draws the hmz2ele preview, then fills each hex with its
// color (see hexColor), keeping the hex outlines. Rivers are drawn along hex
// edges.
func RenderPreview(g hmz2ele.Grid, centers []hmz2ele.Hex, hexes []Hex, rivers []hmz2riv.EdgeJSON, scale int) (*image.RGBA, error) {
	img, err := hmz2ele.RenderPreview(g, centers, scale)
	if err != nil {
		return nil, err
	}
	fill := make(map[int]color.RGBA, len(hexes))
	for _, h := range hexes {
		fill[Slot(g, h.Col, h.Row)] = hexColor(h)
	}
	slotAt := func(px, py int) int {
		col, row := g.HexAt((float64(px)+0.5)*float64(scale), (float64(py)+0.5)*float64(scale))
		if !g.Contains(col, row) {
			return -1
		}
		return Slot(g, col, row)
	}
	b := img.Bounds()
	owner := make([]int, b.Dx()*b.Dy())
	for py := range b.Dy() {
		for px := range b.Dx() {
			owner[py*b.Dx()+px] = slotAt(px, py)
		}
	}
	for py := range b.Dy() {
		for px := range b.Dx() {
			s := owner[py*b.Dx()+px]
			c, ok := fill[s]
			if s < 0 || !ok {
				continue
			}
			// Keep hmz2ele's outlines: an outline pixel borders another hex
			// to its east or south.
			if (px+1 < b.Dx() && owner[py*b.Dx()+px+1] != s) || (py+1 < b.Dy() && owner[(py+1)*b.Dx()+px] != s) {
				c = darken(c)
			}
			img.SetRGBA(px, py, c)
		}
	}
	for _, e := range rivers {
		x1, y1 := g.Vertex(hmz2ele.VertexKey{Col: e.From.Col, Row: e.From.Row, Corner: e.From.Corner})
		x2, y2 := g.Vertex(hmz2ele.VertexKey{Col: e.To.Col, Row: e.To.Row, Corner: e.To.Corner})
		drawLine(img, x1/float64(scale), y1/float64(scale), x2/float64(scale), y2/float64(scale), riverColor)
	}
	return img, nil
}

func darken(c color.RGBA) color.RGBA {
	scale := func(v uint8) uint8 { return uint8(uint16(v) * 3 / 4) }
	return color.RGBA{R: scale(c.R), G: scale(c.G), B: scale(c.B), A: c.A}
}

// drawLine draws a one-pixel line.
func drawLine(img *image.RGBA, x1, y1, x2, y2 float64, c color.RGBA) {
	steps := int(math.Ceil(max(math.Abs(x2-x1), math.Abs(y2-y1)))) + 1
	for s := 0; s <= steps; s++ {
		t := float64(s) / float64(steps)
		p := image.Point{int(x1 + t*(x2-x1)), int(y1 + t*(y2-y1))}
		if p.In(img.Rect) {
			img.SetRGBA(p.X, p.Y, c)
		}
	}
}
