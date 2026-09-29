// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2ter

import (
	"fmt"
	"math"
)

// Level is a connected surface of pixels, 4-connected, that all have the same
// elevation above 0 m and together cover at least one hex's area.
//
// The source is a surface model whose water bodies were flattened, so lakes
// are levels. So are wide rivers, coastal wetlands, salt pans, and patches of
// filled voids, which is why lakes are chosen by seed points.
type Level struct {
	// Lake is the name of the lake whose seed lies on the level, or "" if
	// no seed does.
	Lake string
	// Seed is the index of the first seed on the level, or -1.
	Seed      int
	Elevation int16
	Pixels    int
	// SlotPixels counts the level's pixels in each slot.
	SlotPixels map[int32]int
}

// HexArea returns the area of one hex in pixels.
func HexArea(apothem int) float64 {
	a := float64(apothem)
	return 2 * math.Sqrt(3) * a * a
}

// FindLevels returns every level of at least minPixels pixels, in raster order
// of their first pixel. seeds maps a pixel index to a lake seed's index in
// names; a level holding a seed is that lake. It is an error for a seed to
// miss every level, or for one level to hold seeds of two different lakes.
func FindLevels(width, height int, data []int16, index []int32, minPixels float64, seeds map[int]int, names []string) ([]Level, error) {
	visited := make([]bool, len(data))
	var levels []Level
	var stack, comp []int
	found := make([]bool, len(names))
	var e int16
	push := func(n int) {
		if !visited[n] && data[n] == e {
			visited[n] = true
			stack = append(stack, n)
		}
	}
	for p := range data {
		if e = data[p]; visited[p] || e <= 0 {
			continue
		}
		visited[p] = true
		stack = append(stack[:0], p)
		comp = comp[:0]
		for len(stack) > 0 {
			q := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			comp = append(comp, q)
			x, y := q%width, q/width
			if x > 0 {
				push(q - 1)
			}
			if x < width-1 {
				push(q + 1)
			}
			if y > 0 {
				push(q - width)
			}
			if y < height-1 {
				push(q + width)
			}
		}
		if float64(len(comp)) < minPixels {
			continue
		}
		f := Level{Seed: -1, Elevation: e, Pixels: len(comp), SlotPixels: map[int32]int{}}
		for _, q := range comp {
			if s := index[q]; s >= 0 {
				f.SlotPixels[s]++
			}
			i, ok := seeds[q]
			if !ok {
				continue
			}
			found[i] = true
			switch {
			case f.Seed < 0:
				f.Seed, f.Lake = i, names[i]
			case names[i] != f.Lake:
				return nil, fmt.Errorf("lake seeds %q and %q lie on the same level, at %d m", f.Lake, names[i], e)
			}
		}
		levels = append(levels, f)
	}
	for i, ok := range found {
		if !ok {
			return nil, fmt.Errorf("lake seed %d (%q) is not on a level of at least %.0f pixels", i+1, names[i], minPixels)
		}
	}
	return levels, nil
}
