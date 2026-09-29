// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2ter

// Rules holds the thresholds that classify hexes. Elevations and relief are
// in meters. Rules about the ground use source units (real meters,
// kilometers, or pixels); rules about play, such as depth, use hexes.
type Rules struct {
	// A land hex's landform comes from its relief (p95 − p5): flats below
	// FlatsBelow, plains below PlainsBelow, rolling plains below
	// RollingBelow, hills below HillsBelow, and mountains otherwise.
	FlatsBelow   int `json:"flats_below_m"`
	PlainsBelow  int `json:"plains_below_m"`
	RollingBelow int `json:"rolling_plains_below_m"`
	HillsBelow   int `json:"hills_below_m"`
	// A land hex with a median elevation of at least PlateauMinMedian and
	// relief below PlateauBelow is a plateau, whatever its relief class.
	PlateauMinMedian int `json:"plateau_min_median_m"`
	PlateauBelow     int `json:"plateau_below_m"`
	// A plateau whose center is within VolcanicRadiusRealKm of a volcano is
	// a volcanic highland. The radius is in real (source) kilometers, not
	// campaign kilometers, because it describes the ground, and so doesn't
	// change with the hex size.
	VolcanicRadiusRealKm float64 `json:"volcanic_radius_real_km"`

	// Salt water up to ShallowMax hexes from land is shallow, up to OpenMax
	// is open, and farther out is deep.
	ShallowMax int `json:"shallow_max_hexes"`
	OpenMax    int `json:"open_max_hexes"`

	// A body of salt water is an inland sea if it has at least
	// InlandMinHexes hexes and is cut off from the open sea by removing
	// the salt water within StraitMax hexes of land.
	StraitMax      int `json:"strait_max_hexes"`
	InlandMinHexes int `json:"inland_min_hexes"`

	// A border cut is a chain of at least BorderMinPixels 8-connected land
	// pixels, each above CliffAbove meters and touching no-data.
	CliffAbove      int16 `json:"cliff_above_m"`
	BorderMinPixels int   `json:"border_min_pixels"`
}

// DefaultRules returns the campaign's rules.
func DefaultRules() Rules {
	return Rules{
		FlatsBelow:           20,
		PlainsBelow:          60,
		RollingBelow:         150,
		HillsBelow:           350,
		PlateauMinMedian:     500,
		PlateauBelow:         150,
		VolcanicRadiusRealKm: 25,
		ShallowMax:           12,
		OpenMax:              19,
		StraitMax:            1,
		InlandMinHexes:       30,
		CliffAbove:           10,
		BorderMinPixels:      1000,
	}
}

// Landform returns the landform for a land hex's relief and median
// elevation. It never returns VolcanicHighlands, which depends on where
// the volcanoes are.
func (r Rules) Landform(relief, median int) Landform {
	if median >= r.PlateauMinMedian && relief < r.PlateauBelow {
		return Plateaus
	}
	switch {
	case relief < r.FlatsBelow:
		return Flats
	case relief < r.PlainsBelow:
		return Plains
	case relief < r.RollingBelow:
		return RollingPlains
	case relief < r.HillsBelow:
		return Hills
	}
	return Mountains
}

// Depth returns the depth band of salt water at the given distance from
// land.
func (r Rules) Depth(distance int) Depth {
	switch {
	case distance <= r.ShallowMax:
		return Shallow
	case distance <= r.OpenMax:
		return Open
	}
	return Deep
}
