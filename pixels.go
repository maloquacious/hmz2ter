// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2ter

import (
	"runtime"
	"slices"
	"sync"

	"github.com/maloquacious/dem2hm"
	"github.com/maloquacious/hmz2ele"
)

// Slot numbers a hex in the grid's Columns × Rows rectangle: row × Columns +
// col.
func Slot(g hmz2ele.Grid, col, row int) int {
	return row*g.Columns + col
}

// HexIndex returns, for every pixel of a width × height raster, the slot of
// the hex that contains the pixel's center, or -1 if that hex is not in the
// grid.
func HexIndex(g hmz2ele.Grid) []int32 {
	idx := make([]int32, g.Width*g.Height)
	parallelRows(g.Height, func(y int) {
		for x := range g.Width {
			col, row := g.HexAt(float64(x)+0.5, float64(y)+0.5)
			s := int32(-1)
			if g.Contains(col, row) {
				s = int32(Slot(g, col, row))
			}
			idx[y*g.Width+x] = s
		}
	})
	return idx
}

// parallelRows calls f for every row in [0, height), spread over the CPUs.
func parallelRows(height int, f func(y int)) {
	var wg sync.WaitGroup
	next := make(chan int, 256)
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for y := range next {
				f(y)
			}
		})
	}
	for y := range height {
		next <- y
	}
	close(next)
	wg.Wait()
}

// PixelStats summarizes every pixel in one hex. Elevations are in meters and
// are zero when the hex has no valid pixels.
type PixelStats struct {
	// Pixels is the number of raster pixels in the hex; hexes on the
	// raster's edge are clipped.
	Pixels int
	// Valid is the number of pixels that are not no-data.
	Valid int
	// Land is the number of valid pixels above 0 m.
	Land int
	// Min, P5, Median, P95, and Max are nearest-rank percentiles of the
	// valid pixels' elevations.
	Min, P5, Median, P95, Max int16
}

// Relief is the spread between the 95th and 5th percentiles.
func (s PixelStats) Relief() int {
	return int(s.P95) - int(s.P5)
}

// ComputeStats returns the statistics of every slot's pixels.
func ComputeStats(g hmz2ele.Grid, data []int16, index []int32) []PixelStats {
	stats := make([]PixelStats, g.Columns*g.Rows)
	start := make([]int, len(stats)+1)
	for p, s := range index {
		if s < 0 {
			continue
		}
		stats[s].Pixels++
		if e := data[p]; e != dem2hm.NoDataPixel16 {
			stats[s].Valid++
			start[s+1]++
			if e > 0 {
				stats[s].Land++
			}
		}
	}
	for s := range stats {
		start[s+1] += start[s]
	}
	// Group the valid elevations by slot, then sort each group.
	values := make([]int16, start[len(stats)])
	fill := slices.Clone(start[:len(stats)])
	for p, s := range index {
		if s < 0 || data[p] == dem2hm.NoDataPixel16 {
			continue
		}
		values[fill[s]] = data[p]
		fill[s]++
	}
	parallelRows(len(stats), func(s int) {
		v := values[start[s]:start[s+1]]
		if len(v) == 0 {
			return
		}
		slices.Sort(v)
		st := &stats[s]
		st.Min, st.Max = v[0], v[len(v)-1]
		st.P5, st.Median, st.P95 = nearestRank(v, 5), nearestRank(v, 50), nearestRank(v, 95)
	})
	return stats
}

// nearestRank returns the p-th percentile of sorted values: the smallest
// value with at least p% of the values at or below it.
func nearestRank(sorted []int16, p int) int16 {
	k := (p*len(sorted)+99)/100 - 1
	return sorted[max(k, 0)]
}
