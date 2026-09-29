// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2ter

import (
	"fmt"
	"math"
	"slices"

	"github.com/maloquacious/hmz2ele"
)

// Landform is a hex's underlying physical geography. The names are the
// game's.
type Landform string

const (
	// LandformEmpty is unset. It is an error in the output.
	LandformEmpty     Landform = ""
	FreshWater        Landform = "fresh-water"
	SaltWater         Landform = "salt-water"
	Flats             Landform = "flats"
	Plains            Landform = "plains"
	RollingPlains     Landform = "rolling-plains"
	Hills             Landform = "hills"
	Mountains         Landform = "mountains"
	Plateaus          Landform = "plateaus"
	VolcanicHighlands Landform = "volcanic-highlands"
	// Cliffs and Badlands are border-cut hexes: impassable by normal means.
	Cliffs   Landform = "cliffs"
	Badlands Landform = "badlands"
)

// IsLand reports whether the landform is passable land shaped by relief.
func (l Landform) IsLand() bool {
	switch l {
	case Flats, Plains, RollingPlains, Hills, Mountains, Plateaus, VolcanicHighlands:
		return true
	}
	return false
}

// Surface is a hex's surface or hydrological condition. hmz2ter sets only
// Clear, on hexes whose surface can't change; the climate step sets the
// rest.
type Surface string

const (
	SurfaceEmpty Surface = ""
	SurfaceClear Surface = "clear"
)

// Biome is a hex's ecology. hmz2ter sets only Clear, on hexes with no
// biome; the climate step sets the rest.
type Biome string

const (
	BiomeEmpty Biome = ""
	BiomeClear Biome = "clear"
)

// Depth is a salt-water hex's depth band, from its distance to land.
type Depth string

const (
	DepthNone Depth = ""
	Shallow   Depth = "shallow"
	Open      Depth = "open"
	Deep      Depth = "deep"
)

// Flag marks a feature that combines with a landform.
type Flag string

const (
	// Coast is land next to salt water, or salt water next to land.
	Coast Flag = "coast"
	// River is land with a river on at least one of its edges.
	River Flag = "river"
	// Volcano is land holding a listed volcano.
	Volcano Flag = "volcano"
	// InlandSea is salt water in a body cut off from the open sea by a
	// narrow strait.
	InlandSea Flag = "inland-sea"
	// Impassable is set on every cliff and badland hex and on the land
	// along a border cut.
	Impassable Flag = "impassable"
	// FlatSurface is land mostly covered by levels that aren't lakes: a
	// wetland, salt pan, wide river, or filled void in the source. It is a
	// hint for the climate step.
	FlatSurface Flag = "flat-surface"
)

// Hex is one hex's terrain.
type Hex struct {
	Col, Row int
	Stats    PixelStats
	// Center is the hex's center elevation, sampled as hmz2ele does, or nil
	// for no-data.
	Center   *int16
	Landform Landform
	Surface  Surface
	Biome    Biome
	Depth    Depth
	// SeaDistance is, for salt water, the number of steps from the nearest
	// hex that isn't salt water, or -1 if there is none. It is 0 for other
	// hexes.
	SeaDistance int
	Flags       []Flag
}

// Inputs are the per-pixel and per-slot facts that Classify combines.
type Inputs struct {
	Grid  hmz2ele.Grid
	Rules Rules
	// Centers are hmz2ele's center samples, one per hex in the grid.
	Centers []hmz2ele.Hex
	// Stats, and the pixel counts below, are indexed by slot.
	Stats []PixelStats
	// LakePixels and FlatPixels count lake pixels and other level pixels.
	LakePixels, FlatPixels []int
	// ForeignPixels and SeaPixels count foreign and other no-data pixels.
	ForeignPixels, SeaPixels []int
	// BorderPixels counts border pixels.
	BorderPixels []int
	// RiverSlots and VolcanoSlots are the slots with a river edge and with
	// a volcano.
	RiverSlots, VolcanoSlots map[int]bool
	// Volcanoes are the volcanoes' positions in raster pixels, and
	// PixelSizeM is the real size of a pixel, in meters.
	Volcanoes  [][2]float64
	PixelSizeM float64
}

// base is what the pixels say a hex is, before border cuts are resolved.
type base int

const (
	baseLand base = iota
	baseLake
	baseCut
	baseSea
)

// Classify returns every hex in the grid, ordered by row, then column.
//
// A hex is fresh water if more than half its pixels are lake. Otherwise it
// is land if its center elevation is above 0 m, as in hmz2ele. Otherwise it
// is a border cut if it has more foreign no-data pixels than other no-data
// pixels, and salt water if not.
//
// A cut next to land becomes cliffs if it also touches water, another cut,
// or the grid's edge, and badlands if land surrounds it. Other cuts become
// salt water.
func Classify(in Inputs) ([]Hex, error) {
	g, rules := in.Grid, in.Rules
	hexes := make([]Hex, len(in.Centers))
	bases := make([]base, len(hexes))
	bySlot := make(map[int]int, len(hexes))
	for i, c := range in.Centers {
		s := Slot(g, c.Col, c.Row)
		st := in.Stats[s]
		if st.Pixels == 0 {
			return nil, fmt.Errorf("hex (%d, %d) has no pixels", c.Col, c.Row)
		}
		hexes[i] = Hex{Col: c.Col, Row: c.Row, Stats: st, Center: c.Center}
		switch {
		case 2*in.LakePixels[s] > st.Pixels:
			bases[i] = baseLake
		case c.IsLand():
			bases[i] = baseLand
		case in.ForeignPixels[s] > in.SeaPixels[s]:
			bases[i] = baseCut
		default:
			bases[i] = baseSea
		}
		bySlot[s] = i
	}
	neighbors := make([][]int, len(hexes))
	for i, h := range hexes {
		for _, d := range hmz2ele.Directions {
			c, r := hmz2ele.Neighbor(h.Col, h.Row, d)
			if g.Contains(c, r) {
				neighbors[i] = append(neighbors[i], bySlot[Slot(g, c, r)])
			}
		}
	}
	onEdge := func(i int) bool { return len(neighbors[i]) < len(hmz2ele.Directions) }

	for i := range hexes {
		h := &hexes[i]
		switch bases[i] {
		case baseLake:
			h.Landform = FreshWater
		case baseLand:
			h.Landform = rules.Landform(h.Stats.Relief(), int(h.Stats.Median))
		case baseSea:
			h.Landform = SaltWater
		case baseCut:
			land, other := false, onEdge(i)
			for _, j := range neighbors[i] {
				land = land || bases[j] == baseLand
				other = other || bases[j] != baseLand
			}
			switch {
			case land && other:
				h.Landform = Cliffs
			case land:
				h.Landform = Badlands
			default:
				h.Landform = SaltWater
			}
		}
		if !h.Landform.IsLand() {
			h.Surface, h.Biome = SurfaceClear, BiomeClear
		}
	}

	// Plateaus near a volcano are volcanic highlands.
	for i := range hexes {
		h := &hexes[i]
		if h.Landform != Plateaus {
			continue
		}
		cx, cy := g.Center(h.Col, h.Row)
		for _, v := range in.Volcanoes {
			if math.Hypot(cx-v[0], cy-v[1])*in.PixelSizeM/1000 <= rules.VolcanicRadiusRealKm {
				h.Landform = VolcanicHighlands
				break
			}
		}
	}

	// Breadth-first search from every hex that isn't salt water, through
	// salt water.
	salt := func(i int) bool { return hexes[i].Landform == SaltWater }
	var queue []int
	for i := range hexes {
		if salt(i) {
			hexes[i].SeaDistance = -1
		} else {
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		for _, j := range neighbors[i] {
			if salt(j) && hexes[j].SeaDistance < 0 {
				hexes[j].SeaDistance = hexes[i].SeaDistance + 1
				queue = append(queue, j)
			}
		}
	}
	for i := range hexes {
		if h := &hexes[i]; salt(i) {
			h.Depth = Deep
			if h.SeaDistance >= 0 {
				h.Depth = rules.Depth(h.SeaDistance)
			}
		}
	}
	inland := inlandSeas(hexes, neighbors, onEdge, rules)

	for i := range hexes {
		h := &hexes[i]
		s := Slot(g, h.Col, h.Row)
		switch {
		case salt(i):
			if h.SeaDistance == 1 {
				h.Flags = append(h.Flags, Coast)
			}
			if inland[i] {
				h.Flags = append(h.Flags, InlandSea)
			}
		case h.Landform == Cliffs, h.Landform == Badlands:
			h.Flags = append(h.Flags, Impassable)
		case h.Landform.IsLand():
			if slices.ContainsFunc(neighbors[i], salt) {
				h.Flags = append(h.Flags, Coast)
			}
			if in.RiverSlots[s] {
				h.Flags = append(h.Flags, River)
			}
			if in.VolcanoSlots[s] {
				h.Flags = append(h.Flags, Volcano)
			}
			if in.BorderPixels[s] > 0 {
				h.Flags = append(h.Flags, Impassable)
			}
			if 2*in.FlatPixels[s] > h.Stats.Pixels {
				h.Flags = append(h.Flags, FlatSurface)
			}
		}
		if h.Landform == LandformEmpty {
			return nil, fmt.Errorf("hex (%d, %d) has no landform", h.Col, h.Row)
		}
	}
	return hexes, nil
}

// inlandSeas reports which salt-water hexes are in an inland sea.
//
// Salt water more than StraitMax hexes from land is wide water. Each
// connected body of wide water is open sea if it reaches the grid's edge and
// a possible inland sea if not. The remaining salt water joins the nearest
// body, stepping through salt water; a hex that bodies reach at the same
// step joins an open body if one of them is open, and otherwise the body
// numbered lowest. Bodies are numbered in hex order of their first hex.
// Salt water that no body reaches forms bodies of its own, numbered after. A body that
// doesn't reach the grid's edge is an inland sea if it has at least
// InlandMinHexes hexes.
func inlandSeas(hexes []Hex, neighbors [][]int, onEdge func(int) bool, rules Rules) []bool {
	salt := func(i int) bool { return hexes[i].Landform == SaltWater }
	body := make([]int, len(hexes))
	for i := range body {
		body[i] = -1
	}
	var open []bool
	// flood labels the connected hexes that pass ok, starting at i, as a
	// new body.
	flood := func(i int, ok func(int) bool) {
		id := len(open)
		open = append(open, false)
		body[i] = id
		stack := []int{i}
		for len(stack) > 0 {
			u := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			open[id] = open[id] || onEdge(u)
			for _, v := range neighbors[u] {
				if body[v] < 0 && ok(v) {
					body[v] = id
					stack = append(stack, v)
				}
			}
		}
	}
	wide := func(i int) bool { return salt(i) && hexes[i].SeaDistance > rules.StraitMax }
	var frontier []int
	for i := range hexes {
		if body[i] < 0 && wide(i) {
			flood(i, wide)
		}
		if body[i] >= 0 {
			frontier = append(frontier, i)
		}
	}
	// Grow the bodies one step at a time through the rest of the salt
	// water.
	for len(frontier) > 0 {
		claims := map[int]int{}
		var next []int
		for _, u := range frontier {
			for _, v := range neighbors[u] {
				if body[v] >= 0 || !salt(v) {
					continue
				}
				c, seen := claims[v]
				b := body[u]
				switch {
				case !seen:
					claims[v] = b
					next = append(next, v)
				case open[b] != open[c]:
					if open[b] {
						claims[v] = b
					}
				case b < c:
					claims[v] = b
				}
			}
		}
		for _, v := range next {
			body[v] = claims[v]
		}
		frontier = next
	}
	for i := range hexes {
		if body[i] < 0 && salt(i) {
			flood(i, func(j int) bool { return salt(j) && body[j] < 0 })
		}
	}
	size := make([]int, len(open))
	for _, b := range body {
		if b >= 0 {
			size[b]++
		}
	}
	// A body grown from a closed core may still touch the grid's edge.
	for i, b := range body {
		if b >= 0 && onEdge(i) {
			open[b] = true
		}
	}
	inland := make([]bool, len(hexes))
	for i, b := range body {
		inland[i] = b >= 0 && !open[b] && size[b] >= rules.InlandMinHexes
	}
	return inland
}
