// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2ter

import (
	"math"

	"github.com/maloquacious/dem2hm"
)

// Terrain is the classified hex grid, as written to JSON.
type Terrain struct {
	Hmz2terVersion string        `json:"hmz2ter_version"`
	Heightmap      HeightmapInfo `json:"heightmap"`
	Grid           GridInfo      `json:"grid"`
	Rivers         RiversInfo    `json:"rivers"`
	Rules          Rules         `json:"rules"`
	Method         MethodInfo    `json:"method"`
	Lakes          []LakeJSON    `json:"lakes"`
	Volcanoes      []VolcanoJSON `json:"volcanoes"`
	Stats          Stats         `json:"stats"`
	Hexes          []HexJSON     `json:"hexes"`
}

// HeightmapInfo identifies the heightmap the terrain was classified from.
type HeightmapInfo struct {
	FileName string          `json:"file_name"`
	Metadata dem2hm.Metadata `json:"metadata"`
}

// GridInfo identifies the hex grid.
type GridInfo struct {
	ApothemPx int     `json:"apothem_px"`
	SidePx    float64 `json:"side_px"`
	Columns   int     `json:"columns"`
	Rows      int     `json:"rows"`
	HexCount  int     `json:"hex_count"`
	HexAreaPx float64 `json:"hex_area_px"`
}

// RiversInfo identifies the hmz2riv output the river flags came from.
type RiversInfo struct {
	FileName       string  `json:"file_name"`
	Hmz2rivVersion string  `json:"hmz2riv_version"`
	ThresholdKm2   float64 `json:"threshold_km2"`
}

// MethodInfo describes the rules that Rules doesn't hold.
type MethodInfo struct {
	Statistics string `json:"statistics"`
	Lakes      string `json:"lakes"`
	Borders    string `json:"borders"`
	Cuts       string `json:"cuts"`
	Sea        string `json:"sea"`
	InlandSeas string `json:"inland_seas"`
}

// LakeJSON is one level chosen as a lake by a seed.
type LakeJSON struct {
	Name       string `json:"name"`
	Seed       Place  `json:"seed"`
	ElevationM int16  `json:"elevation_m"`
	Pixels     int    `json:"pixels"`
}

// VolcanoJSON is a listed volcano and the hex that holds it.
type VolcanoJSON struct {
	Place
	Col int `json:"col"`
	Row int `json:"row"`
}

// Stats counts the hexes. SeaDistance[d] is the number of salt-water hexes
// d steps from the nearest hex that isn't salt water; UnreachableSea counts
// salt-water hexes that no such hex reaches.
type Stats struct {
	Landforms      map[Landform]int `json:"landforms"`
	Depths         map[Depth]int    `json:"depths"`
	Flags          map[Flag]int     `json:"flags"`
	SeaDistance    []int            `json:"sea_distance"`
	UnreachableSea int              `json:"unreachable_sea"`
	BorderPixels   int              `json:"border_pixels"`
	ForeignPixels  int              `json:"foreign_pixels"`
	OtherLevels    int              `json:"other_levels"`
}

// ElevationJSON is a hex's elevation percentiles, in meters.
type ElevationJSON struct {
	Min    int16 `json:"min"`
	P5     int16 `json:"p5"`
	Median int16 `json:"median"`
	P95    int16 `json:"p95"`
	Max    int16 `json:"max"`
}

// HexJSON is one hex. Surface and Biome are omitted while unset; the climate
// step sets them for land. Elevation and ReliefM are nil for a hex with no
// valid pixels.
type HexJSON struct {
	Col          int            `json:"col"`
	Row          int            `json:"row"`
	Landform     Landform       `json:"landform"`
	Surface      Surface        `json:"surface,omitempty"`
	Biome        Biome          `json:"biome,omitempty"`
	Depth        Depth          `json:"depth,omitempty"`
	SeaDistance  *int           `json:"sea_distance,omitempty"`
	Flags        []Flag         `json:"flags,omitempty"`
	Center       *int16         `json:"center"`
	Pixels       int            `json:"pixels"`
	ValidPixels  int            `json:"valid_pixels"`
	LandFraction float64        `json:"land_fraction"`
	Elevation    *ElevationJSON `json:"elevation"`
	ReliefM      *int           `json:"relief_m"`
}

// HexesJSON converts hexes for output.
func HexesJSON(hexes []Hex) []HexJSON {
	out := make([]HexJSON, len(hexes))
	for i, h := range hexes {
		st := h.Stats
		j := HexJSON{
			Col:          h.Col,
			Row:          h.Row,
			Landform:     h.Landform,
			Surface:      h.Surface,
			Biome:        h.Biome,
			Depth:        h.Depth,
			Flags:        h.Flags,
			Center:       h.Center,
			Pixels:       st.Pixels,
			ValidPixels:  st.Valid,
			LandFraction: math.Round(float64(st.Land)/float64(st.Pixels)*1e4) / 1e4,
		}
		if h.Landform == SaltWater && h.SeaDistance >= 0 {
			j.SeaDistance = new(h.SeaDistance)
		}
		if st.Valid > 0 {
			j.Elevation = &ElevationJSON{Min: st.Min, P5: st.P5, Median: st.Median, P95: st.P95, Max: st.Max}
			j.ReliefM = new(st.Relief())
		}
		out[i] = j
	}
	return out
}

// Summarize counts the hexes.
func Summarize(hexes []Hex) Stats {
	s := Stats{Landforms: map[Landform]int{}, Depths: map[Depth]int{}, Flags: map[Flag]int{}}
	for _, h := range hexes {
		s.Landforms[h.Landform]++
		if h.Depth != DepthNone {
			s.Depths[h.Depth]++
		}
		for _, f := range h.Flags {
			s.Flags[f]++
		}
		if h.Landform != SaltWater {
			continue
		}
		if h.SeaDistance < 0 {
			s.UnreachableSea++
			continue
		}
		for len(s.SeaDistance) <= h.SeaDistance {
			s.SeaDistance = append(s.SeaDistance, 0)
		}
		s.SeaDistance[h.SeaDistance]++
	}
	return s
}
