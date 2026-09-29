// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Command hmz2ter classifies each hex of a flat-top hex grid over a dem2hm
// heightmap (format 1.1) by landform and water, sets flags such as coast,
// cliff, and river, and writes the result as JSON, with an optional PNG
// preview.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/maloquacious/dem2hm"
	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2riv"
	"github.com/maloquacious/hmz2ter"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "hmz2ter: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("hmz2ter", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hmz2ter [flags] <input.hmz>\n\n")
		fs.PrintDefaults()
	}
	apothem := fs.Int("apothem", 48, "hex apothem in raster pixels (a hex is twice this tall)")
	riversFile := fs.String("rivers", "", "hmz2riv JSON file for the river flags (optional)")
	output := fs.String("output", "", "JSON file to write (required)")
	preview := fs.String("preview", "", "PNG preview file to write (optional)")
	previewScale := fs.Int("preview-scale", 8, "raster pixels per preview pixel, in each direction")
	var lakes, volcanoes []hmz2ter.Place
	placeFlag := func(list *[]hmz2ter.Place) func(string) error {
		return func(s string) error {
			p, err := hmz2ter.ParsePlace(s)
			if err == nil {
				*list = append(*list, p)
			}
			return err
		}
	}
	fs.Func("lake", "`name,lon,lat` of a point on a lake's surface (repeatable)", placeFlag(&lakes))
	fs.Func("volcano", "`name,lon,lat` of a volcano (repeatable)", placeFlag(&volcanoes))
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, hmz2ter.Version())
		return nil
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected one input file, got %d", fs.NArg())
	}
	if *output == "" {
		return fmt.Errorf("-output is required")
	}
	input := fs.Arg(0)
	rules := hmz2ter.DefaultRules()
	start := time.Now()
	phase := func(name string) {
		fmt.Fprintf(stdout, "%-24s %6.1fs\n", name, time.Since(start).Seconds())
	}

	hm, err := readHeightMap(input)
	if err != nil {
		return err
	}
	var rivers hmz2riv.Rivers
	if *riversFile != "" {
		if err := readJSON(*riversFile, &rivers); err != nil {
			return err
		}
	}
	phase("read input")
	g, err := hmz2ele.NewGrid(*apothem, hm.Width, hm.Height)
	if err != nil {
		return err
	}
	slots := g.Columns * g.Rows
	centers := hmz2ele.Build(hmz2ele.RasterFromHeightMap(hm), g)
	index := hmz2ter.HexIndex(g)
	stats := hmz2ter.ComputeStats(g, hm.Data, index)
	phase("hex statistics")

	seeds := map[int]int{}
	names := make([]string, len(lakes))
	for i, l := range lakes {
		p, err := l.Pixel(hm)
		if err != nil {
			return err
		}
		seeds[p], names[i] = i, l.Name
	}
	hexArea := hmz2ter.HexArea(g.Apothem)
	levels, err := hmz2ter.FindLevels(hm.Width, hm.Height, hm.Data, index, hexArea, seeds, names)
	if err != nil {
		return err
	}
	lakePixels, flatPixels := make([]int, slots), make([]int, slots)
	var lakeDocs []hmz2ter.LakeJSON
	otherLevels := 0
	for _, f := range levels {
		count := flatPixels
		if f.Lake != "" {
			count = lakePixels
			lakeDocs = append(lakeDocs, hmz2ter.LakeJSON{Name: f.Lake, Seed: lakes[f.Seed], ElevationM: f.Elevation, Pixels: f.Pixels})
		} else {
			otherLevels++
		}
		for s, n := range f.SlotPixels {
			count[s] += n
		}
	}
	phase("levels and lakes")

	border, shore := hmz2ter.Borders(hm.Width, hm.Height, hm.Data, rules.CliffAbove, rules.BorderMinPixels)
	foreign := hmz2ter.ForeignPixels(hm.Width, hm.Height, hm.Data, border, shore)
	borderPixels, foreignPixels, seaPixels := make([]int, slots), make([]int, slots), make([]int, slots)
	totalBorder, totalForeign := 0, 0
	for p, s := range index {
		if border[p] {
			totalBorder++
		}
		if foreign[p] {
			totalForeign++
		}
		if s < 0 {
			continue
		}
		switch {
		case border[p]:
			borderPixels[s]++
		case foreign[p]:
			foreignPixels[s]++
		case hm.Data[p] == dem2hm.NoDataPixel16:
			seaPixels[s]++
		}
	}
	phase("border cuts")

	riverSlots := map[int]bool{}
	if *riversFile != "" {
		if riverSlots, err = hmz2ter.RiverSlots(g, &rivers); err != nil {
			return fmt.Errorf("%s: %w", *riversFile, err)
		}
	}
	volcanoSlots := map[int]bool{}
	var volcanoPoints [][2]float64
	var volcanoDocs []hmz2ter.VolcanoJSON
	for _, v := range volcanoes {
		p, err := v.Pixel(hm)
		if err != nil {
			return err
		}
		s := int(index[p])
		if s < 0 {
			return fmt.Errorf("volcano %s is outside the grid", v.Name)
		}
		volcanoSlots[s] = true
		x, y, err := hmz2riv.PixelFromLonLat(hm, v.Lon, v.Lat)
		if err != nil {
			return err
		}
		volcanoPoints = append(volcanoPoints, [2]float64{x, y})
		volcanoDocs = append(volcanoDocs, hmz2ter.VolcanoJSON{Place: v, Col: s % g.Columns, Row: s / g.Columns})
	}

	hexes, err := hmz2ter.Classify(hmz2ter.Inputs{
		Grid:          g,
		Rules:         rules,
		Centers:       centers,
		Stats:         stats,
		LakePixels:    lakePixels,
		FlatPixels:    flatPixels,
		ForeignPixels: foreignPixels,
		SeaPixels:     seaPixels,
		BorderPixels:  borderPixels,
		RiverSlots:    riverSlots,
		VolcanoSlots:  volcanoSlots,
		Volcanoes:     volcanoPoints,
		PixelSizeM:    hm.Metadata.PixelSizeMeters,
	})
	if err != nil {
		return err
	}
	for _, v := range volcanoDocs {
		for _, h := range hexes {
			if h.Col == v.Col && h.Row == v.Row && !h.Landform.IsLand() {
				return fmt.Errorf("volcano %s is in a %s hex (%d, %d), not land", v.Name, h.Landform, h.Col, h.Row)
			}
		}
	}
	phase("classify")

	summary := hmz2ter.Summarize(hexes)
	summary.BorderPixels, summary.ForeignPixels, summary.OtherLevels = totalBorder, totalForeign, otherLevels
	doc := hmz2ter.Terrain{
		Hmz2terVersion: hmz2ter.Version().String(),
		Heightmap:      hmz2ter.HeightmapInfo{FileName: filepath.Base(input), Metadata: hm.Metadata},
		Grid: hmz2ter.GridInfo{
			ApothemPx: g.Apothem,
			SidePx:    g.Side,
			Columns:   g.Columns,
			Rows:      g.Rows,
			HexCount:  len(hexes),
			HexAreaPx: hexArea,
		},
		Rules: rules,
		Method: hmz2ter.MethodInfo{
			Statistics: "every pixel whose center lies in the hex; nearest-rank percentiles of the valid pixels; relief = p95 − p5; land fraction = valid pixels above 0 m ÷ pixels",
			Lakes:      "a lake is a level (4-connected pixels of one elevation above 0 m, at least one hex in area) holding a seed; a hex is a lake if more than half its pixels are lake",
			Borders:    "border pixels are chains of 8-connected land pixels above cliff_above_m that touch no-data, at least border_min_pixels long; no-data closer (chessboard) to a border pixel than to any other edge pixel is foreign; a hex with more foreign than other no-data pixels, and not land or lake, is a border cut",
			Cuts:       "a cut next to land is cliffs if it also touches water, another cut, or the grid's edge, and badlands if not; other cuts are salt water",
			Sea:        "breadth-first hex steps from the nearest hex that isn't salt water, through salt water only",
			InlandSeas: "salt water more than strait_max_hexes from land forms bodies, open if they reach the grid's edge; the rest of the salt water joins the nearest body (open first, then lowest number); a closed body of at least inland_min_hexes is an inland sea",
		},
		Lakes:     lakeDocs,
		Volcanoes: volcanoDocs,
		Stats:     summary,
		Hexes:     hmz2ter.HexesJSON(hexes),
	}
	if *riversFile != "" {
		doc.Rivers = hmz2ter.RiversInfo{
			FileName:       filepath.Base(*riversFile),
			Hmz2rivVersion: rivers.Hmz2rivVersion,
			ThresholdKm2:   rivers.Hydrology.ThresholdKm2,
		}
	}
	if err := writeFile(*output, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	}); err != nil {
		return err
	}
	if *preview != "" {
		img, err := hmz2ter.RenderPreview(g, centers, hexes, rivers.Edges, *previewScale)
		if err != nil {
			return err
		}
		if err := writeFile(*preview, func(w io.Writer) error { return png.Encode(w, img) }); err != nil {
			return err
		}
	}
	phase("write output")

	fmt.Fprintf(stdout, "hexes:              %d\n", len(hexes))
	fmt.Fprintf(stdout, "landforms:\n")
	for _, l := range []hmz2ter.Landform{hmz2ter.FreshWater, hmz2ter.SaltWater, hmz2ter.Flats, hmz2ter.Plains, hmz2ter.RollingPlains, hmz2ter.Hills, hmz2ter.Mountains, hmz2ter.Plateaus, hmz2ter.VolcanicHighlands, hmz2ter.Cliffs, hmz2ter.Badlands} {
		fmt.Fprintf(stdout, "  %-20s %d\n", l+":", summary.Landforms[l])
	}
	fmt.Fprintf(stdout, "salt-water depths:\n")
	for _, d := range []hmz2ter.Depth{hmz2ter.Shallow, hmz2ter.Open, hmz2ter.Deep} {
		fmt.Fprintf(stdout, "  %-20s %d\n", d+":", summary.Depths[d])
	}
	fmt.Fprintf(stdout, "flags:\n")
	for _, f := range []hmz2ter.Flag{hmz2ter.Coast, hmz2ter.River, hmz2ter.Volcano, hmz2ter.InlandSea, hmz2ter.Impassable, hmz2ter.FlatSurface} {
		fmt.Fprintf(stdout, "  %-20s %d\n", f+":", summary.Flags[f])
	}
	fmt.Fprintf(stdout, "lakes:              %d levels (%d other levels of at least one hex)\n", len(lakeDocs), otherLevels)
	fmt.Fprintf(stdout, "border pixels:      %d (%d foreign no-data pixels)\n", totalBorder, totalForeign)
	fmt.Fprintf(stdout, "unreachable sea:    %d\n", summary.UnreachableSea)
	fmt.Fprintf(stdout, "sea distance:      ")
	for d, n := range summary.SeaDistance {
		if d > 0 {
			fmt.Fprintf(stdout, " %d:%d", d, n)
		}
	}
	fmt.Fprintln(stdout)
	return nil
}

func readHeightMap(path string) (*dem2hm.HeightMap16, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hm, err := dem2hm.ReadHeightMap16(bufio.NewReader(f))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return hm, nil
}

func readJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(bufio.NewReader(f)).Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func writeFile(path string, write func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	if err := write(w); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	return f.Close()
}
