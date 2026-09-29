// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2ter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/maloquacious/dem2hm"
	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2riv"
)

// Place is a named point, such as a lake seed or a volcano.
type Place struct {
	Name string  `json:"name"`
	Lon  float64 `json:"lon"`
	Lat  float64 `json:"lat"`
}

// ParsePlace parses "name,lon,lat".
func ParsePlace(s string) (Place, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" {
		return Place{}, fmt.Errorf("%q: want name,lon,lat", s)
	}
	var f [2]float64
	for i := range f {
		v, err := strconv.ParseFloat(strings.TrimSpace(parts[i+1]), 64)
		if err != nil {
			return Place{}, fmt.Errorf("%q: %w", s, err)
		}
		f[i] = v
	}
	return Place{Name: strings.TrimSpace(parts[0]), Lon: f[0], Lat: f[1]}, nil
}

func (p Place) String() string {
	return fmt.Sprintf("%s,%g,%g", p.Name, p.Lon, p.Lat)
}

// Pixel returns the index of the heightmap pixel under the place.
func (p Place) Pixel(hm *dem2hm.HeightMap16) (int, error) {
	x, y, err := hmz2riv.PixelFromLonLat(hm, p.Lon, p.Lat)
	if err != nil {
		return 0, err
	}
	px, py := int(x), int(y)
	if x < 0 || y < 0 || px >= hm.Width || py >= hm.Height {
		return 0, fmt.Errorf("%s: %g,%g is outside the heightmap", p.Name, p.Lon, p.Lat)
	}
	return py*hm.Width + px, nil
}

// RiverSlots returns the slots of every hex on either side of a river edge.
func RiverSlots(g hmz2ele.Grid, rivers *hmz2riv.Rivers) (map[int]bool, error) {
	if rivers.Grid.ApothemPx != g.Apothem || rivers.Grid.Columns != g.Columns || rivers.Grid.Rows != g.Rows {
		return nil, fmt.Errorf("rivers grid is apothem %d, %d × %d; want apothem %d, %d × %d",
			rivers.Grid.ApothemPx, rivers.Grid.Columns, rivers.Grid.Rows, g.Apothem, g.Columns, g.Rows)
	}
	across := map[hmz2ele.Side]hmz2ele.Direction{
		hmz2ele.NorthSide:     hmz2ele.North,
		hmz2ele.NorthEastSide: hmz2ele.NorthEast,
		hmz2ele.SouthEastSide: hmz2ele.SouthEast,
	}
	slots := map[int]bool{}
	for _, e := range rivers.Edges {
		d, ok := across[e.Side]
		if !ok {
			return nil, fmt.Errorf("river edge (%d, %d): invalid side %v", e.Col, e.Row, e.Side)
		}
		nc, nr := hmz2ele.Neighbor(e.Col, e.Row, d)
		for _, h := range [2][2]int{{e.Col, e.Row}, {nc, nr}} {
			if g.Contains(h[0], h[1]) {
				slots[Slot(g, h[0], h[1])] = true
			}
		}
	}
	return slots, nil
}
