// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2ter

import (
	"github.com/maloquacious/dem2hm"
)

// Borders finds where the raster was cut along a national border rather
// than along a coast.
//
// An edge pixel is a valid pixel with a no-data pixel among its 8 neighbors.
// A cliff pixel is an edge pixel above cliffAbove meters. The source was cut
// to the country's borders, so land there stops above sea level; sea cliffs
// exist too, but they are short. Border pixels are the cliff pixels in
// 8-connected chains of at least minPixels.
//
// Borders returns the border pixels and the other edge pixels, the shore.
func Borders(width, height int, data []int16, cliffAbove int16, minPixels int) (border, shore []bool) {
	noData := func(x, y int) bool {
		return x >= 0 && y >= 0 && x < width && y < height && data[y*width+x] == dem2hm.NoDataPixel16
	}
	edge := make([]bool, len(data))
	parallelRows(height, func(y int) {
		for x := range width {
			if data[y*width+x] == dem2hm.NoDataPixel16 {
				continue
			}
			for dy := -1; dy <= 1 && !edge[y*width+x]; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if noData(x+dx, y+dy) {
						edge[y*width+x] = true
						break
					}
				}
			}
		}
	})

	cliff := func(p int) bool { return edge[p] && data[p] > cliffAbove }
	border = make([]bool, len(data))
	visited := make([]bool, len(data))
	var stack, chain []int
	for p := range data {
		if visited[p] || !cliff(p) {
			continue
		}
		visited[p] = true
		stack = append(stack[:0], p)
		chain = chain[:0]
		for len(stack) > 0 {
			q := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			chain = append(chain, q)
			x, y := q%width, q/width
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= width || ny >= height {
						continue
					}
					if n := ny*width + nx; !visited[n] && cliff(n) {
						visited[n] = true
						stack = append(stack, n)
					}
				}
			}
		}
		if len(chain) >= minPixels {
			for _, q := range chain {
				border[q] = true
			}
		}
	}
	for p := range edge {
		edge[p] = edge[p] && !border[p]
	}
	return border, edge
}

// ForeignPixels reports, for each pixel, whether it is no-data on the far side of
// a border cut: foreign land, not sea. A no-data pixel is foreign when it is
// strictly closer to a border pixel than to a shore pixel, measuring
// distance in the chessboard metric (8-neighbor steps) straight across the
// raster.
func ForeignPixels(width, height int, data []int16, border, shore []bool) []bool {
	toBorder := chessboard(width, height, border)
	toShore := chessboard(width, height, shore)
	foreign := make([]bool, len(data))
	for p, e := range data {
		foreign[p] = e == dem2hm.NoDataPixel16 && toBorder[p] < toShore[p]
	}
	return foreign
}

const farAway = ^uint16(0)

// chessboard returns each pixel's chessboard distance to the nearest source
// pixel, or farAway if there are none. Two raster passes with the 3 × 3
// neighborhood give the exact distance.
func chessboard(width, height int, source []bool) []uint16 {
	d := make([]uint16, len(source))
	for p, s := range source {
		if !s {
			d[p] = farAway
		}
	}
	relax := func(p, x, y int) {
		if x < 0 || y < 0 || x >= width || y >= height {
			return
		}
		if n := d[y*width+x]; n != farAway && n+1 < d[p] {
			d[p] = n + 1
		}
	}
	for y := range height {
		for x := range width {
			p := y*width + x
			relax(p, x-1, y)
			relax(p, x-1, y-1)
			relax(p, x, y-1)
			relax(p, x+1, y-1)
		}
	}
	for y := height - 1; y >= 0; y-- {
		for x := width - 1; x >= 0; x-- {
			p := y*width + x
			relax(p, x+1, y)
			relax(p, x+1, y+1)
			relax(p, x, y+1)
			relax(p, x-1, y+1)
		}
	}
	return d
}
