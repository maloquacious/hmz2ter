// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2ter

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/maloquacious/dem2hm"
	"github.com/maloquacious/hmz2ele"
)

const nd = dem2hm.NoDataPixel16

func TestNearestRank(t *testing.T) {
	v := []int16{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	for _, tc := range []struct {
		p    int
		want int16
	}{{0, 10}, {5, 10}, {10, 10}, {11, 20}, {50, 50}, {95, 100}, {100, 100}} {
		if got := nearestRank(v, tc.p); got != tc.want {
			t.Errorf("nearestRank(p=%d) = %d, want %d", tc.p, got, tc.want)
		}
	}
	if got := nearestRank([]int16{7}, 5); got != 7 {
		t.Errorf("nearestRank of one value = %d, want 7", got)
	}
}

// A regular hex grid's hexes are the Voronoi cells of their centers, so
// every pixel belongs to the hex with the nearest center.
func TestHexIndexIsNearestCenter(t *testing.T) {
	g, err := hmz2ele.NewGrid(6, 60, 50)
	if err != nil {
		t.Fatal(err)
	}
	idx := HexIndex(g)
	for y := range g.Height {
		for x := range g.Width {
			px, py := float64(x)+0.5, float64(y)+0.5
			best, bestD := -1, math.Inf(1)
			for row := -1; row <= g.Rows; row++ {
				for col := -1; col <= g.Columns; col++ {
					cx, cy := g.Center(col, row)
					if d := math.Hypot(px-cx, py-cy); d < bestD {
						best, bestD = -1, d
						if g.Contains(col, row) {
							best = Slot(g, col, row)
						}
					}
				}
			}
			if got := int(idx[y*g.Width+x]); got != best {
				t.Fatalf("pixel (%d, %d): slot %d, want %d", x, y, got, best)
			}
		}
	}
}

func TestComputeStats(t *testing.T) {
	g, err := hmz2ele.NewGrid(4, 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	idx := HexIndex(g)
	data := make([]int16, g.Width*g.Height)
	var want PixelStats
	var values []int16
	s0 := Slot(g, 0, 0)
	for p := range data {
		data[p] = int16(p%11) - 3
		if p%5 == 0 {
			data[p] = nd
		}
		if idx[p] != int32(s0) {
			continue
		}
		want.Pixels++
		if data[p] != nd {
			want.Valid++
			values = append(values, data[p])
			if data[p] > 0 {
				want.Land++
			}
		}
	}
	got := ComputeStats(g, data, idx)[s0]
	if got.Pixels != want.Pixels || got.Valid != want.Valid || got.Land != want.Land {
		t.Fatalf("counts = %d/%d/%d, want %d/%d/%d", got.Pixels, got.Valid, got.Land, want.Pixels, want.Valid, want.Land)
	}
	lo, hi := values[0], values[0]
	for _, v := range values {
		lo, hi = min(lo, v), max(hi, v)
	}
	if got.Min != lo || got.Max != hi || got.P5 > got.Median || got.Median > got.P95 {
		t.Errorf("stats %+v inconsistent with min %d max %d", got, lo, hi)
	}
}

func TestChessboardMatchesBruteForce(t *testing.T) {
	const w, h = 23, 17
	r := rand.New(rand.NewPCG(1, 2))
	src := make([]bool, w*h)
	for i := range src {
		src[i] = r.IntN(40) == 0
	}
	d := chessboard(w, h, src)
	for p := range src {
		want := int(farAway)
		for q, s := range src {
			if s {
				dx, dy := abs(p%w-q%w), abs(p/w-q/w)
				want = min(want, max(dx, dy))
			}
		}
		if int(d[p]) != want {
			t.Fatalf("pixel %d: distance %d, want %d", p, d[p], want)
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// A raster whose top half is no-data: the long high edge along the middle
// is a border cut, and the far side is foreign; a short high bump on the
// shore is not.
func TestBordersAndForeign(t *testing.T) {
	const w, h = 40, 20
	data := make([]int16, w*h)
	for y := range h {
		for x := range w {
			switch {
			case y < 10 && x < 30:
				data[y*w+x] = nd // foreign, across the cut
			case x >= 30:
				data[y*w+x] = nd // sea to the east
			case y == 10 && x < 25:
				data[y*w+x] = 200 // land cut at the border
			default:
				data[y*w+x] = 5 // low coast
			}
		}
	}
	data[15*w+29] = 80 // a one-pixel sea cliff
	border, shore := Borders(w, h, data, 10, 20)
	for x := range 25 {
		if !border[10*w+x] {
			t.Errorf("pixel (%d, 10) is not a border pixel", x)
		}
	}
	if border[15*w+29] || !shore[15*w+29] {
		t.Error("the short sea cliff is a border pixel, want shore")
	}
	foreign := ForeignPixels(w, h, data, border, shore)
	if !foreign[2*w+5] {
		t.Error("no-data across the border is not foreign")
	}
	if foreign[15*w+38] {
		t.Error("the sea is foreign")
	}
}

func TestFindLevels(t *testing.T) {
	const w, h = 10, 10
	data := make([]int16, w*h)
	idx := make([]int32, w*h)
	for p := range data {
		data[p] = int16(p) + 100 // every pixel different
	}
	for y := 2; y < 6; y++ {
		for x := 2; x < 7; x++ {
			data[y*w+x] = 30 // a 20-pixel lake
		}
	}
	data[9*w+0], data[9*w+1] = 0, 0 // sea level is never a level
	names := []string{"Pond"}
	levels, err := FindLevels(w, h, data, idx, 20, map[int]int{3*w + 3: 0}, names)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 1 || levels[0].Lake != "Pond" || levels[0].Pixels != 20 || levels[0].Elevation != 30 {
		t.Fatalf("levels = %+v, want one 20-pixel lake at 30 m", levels)
	}
	if _, err := FindLevels(w, h, data, idx, 21, map[int]int{3*w + 3: 0}, names); err == nil {
		t.Error("a seed on a level smaller than the minimum is not an error")
	}
	if _, err := FindLevels(w, h, data, idx, 20, map[int]int{3*w + 3: 0, 4*w + 4: 1}, []string{"A", "B"}); err == nil {
		t.Error("two lakes' seeds on one level is not an error")
	}
}

func TestRules(t *testing.T) {
	r := DefaultRules()
	for _, tc := range []struct {
		relief, median int
		want           Landform
	}{
		{0, 10, Flats}, {19, 10, Flats}, {20, 10, Plains}, {59, 10, Plains}, {60, 10, RollingPlains},
		{149, 10, RollingPlains}, {150, 10, Hills}, {349, 10, Hills}, {350, 10, Mountains},
		{149, 500, Plateaus}, {150, 500, Hills}, {10, 499, Flats},
	} {
		if got := r.Landform(tc.relief, tc.median); got != tc.want {
			t.Errorf("Landform(%d, %d) = %s, want %s", tc.relief, tc.median, got, tc.want)
		}
	}
	for d, want := range map[int]Depth{1: Shallow, 12: Shallow, 13: Open, 19: Open, 20: Deep, 41: Deep} {
		if got := r.Depth(d); got != want {
			t.Errorf("Depth(%d) = %s, want %s", d, got, want)
		}
	}
}

// testInputs builds Classify inputs for a 13 × 12 grid whose hexes are
// land ('L'), lake ('K'), border cut ('C'), or sea ('S'), as kind says.
// Land is 50 m and flat.
func testInputs(t *testing.T, kind func(col, row int) byte) Inputs {
	t.Helper()
	return testInputsSized(t, 13, 12, kind)
}

// testInputsSized is testInputs for a grid of the given size.
func testInputsSized(t *testing.T, columns, rows int, kind func(col, row int) byte) Inputs {
	t.Helper()
	side := 8 / math.Sqrt(3)
	g, err := hmz2ele.NewGrid(4, int(side+1.5*side*float64(columns-1))+1, 8*rows)
	if err != nil {
		t.Fatal(err)
	}
	if g.Columns != columns || g.Rows != rows {
		t.Fatalf("grid %d × %d, want %d × %d", g.Columns, g.Rows, columns, rows)
	}
	n := g.Columns * g.Rows
	in := Inputs{
		Grid: g, Rules: DefaultRules(), Stats: make([]PixelStats, n),
		LakePixels: make([]int, n), FlatPixels: make([]int, n),
		ForeignPixels: make([]int, n), SeaPixels: make([]int, n), BorderPixels: make([]int, n),
		RiverSlots: map[int]bool{}, VolcanoSlots: map[int]bool{},
	}
	for row := range g.Rows {
		for col := range g.Columns {
			if !g.Contains(col, row) {
				continue
			}
			s := Slot(g, col, row)
			in.Stats[s] = PixelStats{Pixels: 100, Valid: 100, Median: 50, P5: 50, P95: 50}
			h := hmz2ele.Hex{Col: col, Row: row}
			switch kind(col, row) {
			case 'L':
				h.Center = new(int16(50))
			case 'K':
				in.LakePixels[s] = 100
			case 'C':
				in.ForeignPixels[s] = 100
			case 'S':
				in.SeaPixels[s] = 100
			}
			in.Centers = append(in.Centers, h)
		}
	}
	return in
}

func landformAt(hexes []Hex, col, row int) Hex {
	for _, h := range hexes {
		if h.Col == col && h.Row == row {
			return h
		}
	}
	panic("no hex")
}

func TestClassifyCuts(t *testing.T) {
	in := testInputs(t, func(col, row int) byte {
		switch {
		case col == 5 && row == 5, col == 2 && row == 2, col >= 10:
			return 'C'
		case col == 2 && row == 1:
			return 'S'
		}
		return 'L'
	})
	hexes, err := Classify(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		col, row int
		want     Landform
	}{
		{5, 5, Badlands},   // surrounded by land
		{2, 2, Cliffs},     // land and sea
		{10, 4, Cliffs},    // land and other cuts
		{11, 4, SaltWater}, // no land
		{12, 4, SaltWater},
		{4, 4, Flats},
	} {
		h := landformAt(hexes, tc.col, tc.row)
		if h.Landform != tc.want {
			t.Errorf("hex (%d, %d) = %s, want %s", tc.col, tc.row, h.Landform, tc.want)
		}
		impassable := slices.Contains(h.Flags, Impassable)
		if want := tc.want == Cliffs || tc.want == Badlands; impassable != want {
			t.Errorf("hex (%d, %d): impassable %v, want %v", tc.col, tc.row, impassable, want)
		}
		if !h.Landform.IsLand() && (h.Surface != SurfaceClear || h.Biome != BiomeClear) {
			t.Errorf("hex (%d, %d): surface %q biome %q, want clear", tc.col, tc.row, h.Surface, h.Biome)
		}
	}
	// Salt water next to a cliff is coast.
	if h := landformAt(hexes, 2, 1); !slices.Contains(h.Flags, Coast) || h.SeaDistance != 1 {
		t.Errorf("sea next to cliffs: %+v, want coast at distance 1", h)
	}
}

// hexDistance is the grid distance between two hexes of an odd-q grid.
func hexDistance(c1, r1, c2, r2 int) int {
	cube := func(c, r int) (int, int) { return c, r - (c-(c&1))/2 }
	q1, s1 := cube(c1, r1)
	q2, s2 := cube(c2, r2)
	dq, dr := q1-q2, s1-s2
	return max(abs(dq), abs(dr), abs(dq+dr))
}

// Open sea in columns 0–4, and a round bay around (15, 9) joined to it by a
// channel width hexes tall, in rows 7 to 7 + width − 1.
func TestInlandSeas(t *testing.T) {
	for _, tc := range []struct {
		width, strait, min int
		want               bool
	}{
		{1, 1, 10, true},
		{1, 1, 200, false},
		{3, 1, 10, false}, // every channel hex is next to land only if it is at most 2 wide
		{3, 2, 10, true},
		{4, 2, 10, true},
		{5, 2, 10, false},
	} {
		kind := func(col, row int) byte {
			if col <= 4 || hexDistance(col, row, 15, 9) <= 4 || (col >= 5 && col <= 11 && row >= 7 && row < 7+tc.width) {
				return 'S'
			}
			return 'L'
		}
		in := testInputsSized(t, 22, 18, kind)
		in.Rules.StraitMax = tc.strait
		in.Rules.InlandMinHexes = tc.min
		hexes, err := Classify(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := slices.Contains(landformAt(hexes, 15, 9).Flags, InlandSea); got != tc.want {
			t.Errorf("channel %d, strait %d, min %d: bay inland %v, want %v", tc.width, tc.strait, tc.min, got, tc.want)
		}
		if slices.Contains(landformAt(hexes, 0, 6).Flags, InlandSea) {
			t.Errorf("channel %d, strait %d, min %d: open sea is inland", tc.width, tc.strait, tc.min)
		}
	}
}

func TestVolcanicHighlands(t *testing.T) {
	in := testInputs(t, func(col, row int) byte { return 'L' })
	for s := range in.Stats {
		in.Stats[s].Median = 600 // every land hex is a plateau
	}
	vx, vy := in.Grid.Center(6, 6)
	in.Volcanoes = [][2]float64{{vx, vy}}
	in.PixelSizeM = 1000 // 1 km per pixel
	in.Rules.VolcanicRadiusRealKm = 17
	hexes, err := Classify(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hexes {
		cx, cy := in.Grid.Center(h.Col, h.Row)
		want := Plateaus
		if math.Hypot(cx-vx, cy-vy) <= 17 {
			want = VolcanicHighlands
		}
		if h.Landform != want {
			t.Errorf("hex (%d, %d) = %s, want %s", h.Col, h.Row, h.Landform, want)
		}
	}
	if landformAt(hexes, 6, 6).Landform != VolcanicHighlands || landformAt(hexes, 0, 0).Landform != Plateaus {
		t.Error("the radius did not separate near hexes from far ones")
	}
}

func TestParsePlace(t *testing.T) {
	p, err := ParsePlace("El Valle, -80.13, 8.6")
	if err != nil || p != (Place{"El Valle", -80.13, 8.6}) {
		t.Errorf("ParsePlace = %+v, %v", p, err)
	}
	for _, s := range []string{"", "a,1", ",1,2", "a,x,2"} {
		if _, err := ParsePlace(s); err == nil {
			t.Errorf("ParsePlace(%q) is not an error", s)
		}
	}
}
