# hmz2ter

`hmz2ter` classifies each hex of the flat-top hex grid used by [`hmz2ele`](https://github.com/maloquacious/hmz2ele) over a `dem2hm` heightmap (format 1.1).
It gives each hex a landform (the game's physical geography), a depth for salt water, and flags such as coast, river, and impassable, and writes the result as JSON, with an optional PNG preview.

It does not assign final game terrain.
Forest, grassland, tundra, desert, wetlands, and so on depend on climate, which is a later step.

## Usage

```text
go run ./cmd/hmz2ter [flags] <input.hmz>
```

Flags:

- `-apothem <px>` sets the hex apothem in raster pixels. The default is `48`.
- `-rivers <file>` is an `hmz2riv` JSON file for the river flags. Optional; its grid must match.
- `-lake <name,lon,lat>` names a point on a lake's surface. Repeatable. See [Lakes](#lakes).
- `-volcano <name,lon,lat>` marks a volcano. Repeatable. The point must be in a land hex.
- `-output <file>` is the JSON file to write. Required.
- `-preview <file>` is a PNG preview to write. Optional.
- `-preview-scale <n>` sets how many raster pixels, in each direction, one preview pixel covers. The default is `8`.
- `-version` prints the version.

The command prints the time taken by each phase and a summary: counts by landform, depth, and flag, and the number of salt-water hexes at each distance from land.
A full run on the Panama heightmap (189 million pixels) takes about 7 seconds.

## Hex statistics

Every pixel whose center lies in a hex counts toward that hex, so the statistics use all of the hex's pixels, not samples.
Hexes on the raster's edge are clipped to the raster.

For each hex:

- **Pixels**, **valid pixels** (not no-data), and **land fraction**: valid pixels above 0 m divided by pixels.
- **Elevation**: the minimum, 5th percentile, median, 95th percentile, and maximum of the valid pixels, in meters. Percentiles are nearest-rank: the p-th percentile of n sorted values is value number ⌈p·n/100⌉.
- **Relief**: the 95th percentile minus the 5th, in meters.

Relief uses the 5th to 95th percentiles instead of the minimum and maximum because the source is a surface model: tree canopy and buildings add spikes that would inflate the full range.

The center elevation is also copied from `hmz2ele`'s 3 × 3 median sample, because it decides which hexes are land.

## Landforms

Each hex gets a landform, using the game's names:

1. **`fresh-water`**: more than half the hex's pixels are lake (see [Lakes](#lakes)).
2. **Land**: the center elevation is above 0 m, the same test `hmz2ele` uses. The landform comes from the relief:

   | Landform         | Relief         |
   | ---------------- | -------------: |
   | `flats`          | under 20 m     |
   | `plains`         | 20 to 60 m     |
   | `rolling-plains` | 60 to 150 m    |
   | `hills`          | 150 to 350 m   |
   | `mountains`      | 350 m and over |

   A land hex with a median elevation of 500 m or more and relief under 150 m is **`plateaus`** instead, and a plateau whose center is within 25 km of a volcano is **`volcanic-highlands`**.
   The 25 km is real (source) distance, about 8.5 hexes at apothem 48, because it describes the ground around the volcano; see [Units](#units).
3. **Border cut**: a hex with more foreign no-data pixels than other no-data pixels (see [Border cuts](#border-cuts)). A cut next to land becomes **`cliffs`** if it also touches water, another cut, or the grid's edge, and **`badlands`** if land surrounds it. Other cuts become salt water.
4. **`salt-water`**: everything else.

There is no alpine landform: the alpine line depends on latitude, so the climate step decides it from the median elevation, as a biome.

Salt water also gets a **depth** from its distance to land, counted in hex steps from the nearest hex that isn't salt water (land, fresh water, cliffs, or badlands), through salt water only:

| Depth     | Distance     |
| --------- | -----------: |
| `shallow` | 1 to 12      |
| `open`    | 13 to 19     |
| `deep`    | 20 and more  |

Salt water that nothing reaches is `deep`.
The source has no sea depths, so depth is distance.

## Units

Rules about the ground use source units, so they keep describing the same ground if the hex size changes: relief and cliff heights in meters, lake size and border chains in pixels, and the volcanic-highlands radius in real kilometers (pixels × the heightmap's `pixel_size_m`).
Rules about play use hexes, so they keep playing the same: depth bands, inland-sea straits, and coast.
Real kilometers are not campaign kilometers: the campaign map is about 3.4 times real scale, and a hex is always 10 campaign km.

## Surface and biome

The climate step sets each land hex's surface and biome, so `hmz2ter` leaves them unset.
Water, cliffs, and badlands have no surface condition and no biome, so `hmz2ter` sets both to `clear`.

## Lakes

A **level** is a set of 4-connected pixels that all have the same elevation above 0 m and together cover at least one hex's area (about 7,981 pixels at apothem 48).
The source flattened its water bodies, so every lake is a level.

Not every level is a lake, though.
On the Panama heightmap there are 66 levels, and only 6 of them are lakes.
The rest are coastal wetlands and salt pans, wide stepped river reaches in Darién, and a few flat patches in the hills that look like filled data voids.
Their shapes and settings don't separate them from lakes reliably, so each lake is chosen by a seed: a `-lake` point on its surface.
A lake can have several levels; Gatun Lake has two (28 m and 29 m) and Lake Bayano has three (59, 60, and 62 m), so each level needs its own seed.
A seed that is not on a level, or two lakes' seeds on one level, is an error.

A hex is a lake if more than half its pixels are lake.
Levels that aren't lakes set the `flat-surface` flag on land instead.

## Border cuts

The source was cut to Panama's borders, so no-data is both sea and foreign land, and land ends at the border in a sheer cliff.

1. An **edge pixel** is a valid pixel with a no-data pixel among its 8 neighbors.
2. A **cliff pixel** is an edge pixel above 10 m.
3. A **border pixel** is a cliff pixel in an 8-connected chain of at least 1,000 cliff pixels. Sea cliffs are short: on the Panama heightmap, the Costa Rica and Colombia cuts are two chains of about 12,800 and 14,100 pixels, and the next-longest chain is 376 pixels.
4. A no-data pixel is **foreign** if it is strictly closer to a border pixel than to any other edge pixel, measured in 8-neighbor steps straight across the raster.

Foreign no-data is the parts of Costa Rica and Colombia inside the raster.
The game world has no neighboring countries, so the border cuts become [cliffs, badlands, or sea](#landforms).

The threshold is 10 m rather than 30 m because at 30 m the Costa Rica cut breaks where it crosses the Pacific lowland near Paso Canoas, at 15 to 30 m, and sea would leak into Costa Rica through the gap.

## Inland seas

An inland sea is a body of salt water cut off from the open sea by a narrow strait.

1. Salt water more than 2 hexes from land is **wide water**. Each connected body of wide water is open sea if it reaches the grid's edge.
2. The rest of the salt water joins the nearest body, stepping through salt water. A hex that two bodies reach at the same step joins an open body if either is open, and otherwise the body numbered lowest; bodies are numbered in hex order.
3. Salt water that no body reaches forms bodies of its own.
4. A body that doesn't reach the grid's edge, with at least 30 hexes, is an inland sea.

So a body's connection to the open sea, if it has one, is a strait whose hexes are all within 2 hexes of land: 4 hexes wide at most, the widest strait small boats can cross.
A narrower cut-off, 1 hex from land, made the result hinge on single hexes at a strait's mouth: moving the hex centers by one apothem turned Laguna de Chiriquí into open sea and Bahía de Almirante into an inland sea.
On the Panama map this finds two inland seas: Laguna de Chiriquí, with 125 hexes, and Golfo de Montijo, with 61.

## Flags

- **`coast`**: land next to salt water, or salt water next to a hex that isn't salt water. Lakes don't make coast. Small boats can travel in coast water and in the water next to it: up to 2 hexes from land.
- **`river`**: land with a river on at least one of its six edges, from the `-rivers` file.
- **`volcano`**: land holding a `-volcano` point.
- **`inland-sea`**: salt water in an inland sea.
- **`impassable`**: every cliff and badland hex, and land holding at least one border pixel. The game treats these hexes as impassable by normal means.
- **`flat-surface`**: land where more than half the pixels are levels that aren't lakes: a hint for the climate step's wetlands, mangroves, and salt flats.

## Output

Elevations are in meters.

```json
{
  "hmz2ter_version": "0.3.0",
  "heightmap": { "file_name": "pandemokh.hmz", "metadata": { ... } },
  "grid": { "apothem_px": 48, "side_px": 55.43, "columns": 106, "rows": 222, "hex_count": 23479, "hex_area_px": 7981.3 },
  "rivers": { "file_name": "pandemokh-a48-rivers.json", "hmz2riv_version": "0.3.0", "threshold_km2": 50 },
  "rules": { "flats_below_m": 20, ..., "open_max_hexes": 19, "inland_min_hexes": 30, "cliff_above_m": 10, "border_min_pixels": 1000 },
  "method": { "statistics": "...", "lakes": "...", "borders": "...", "cuts": "...", "sea": "...", "inland_seas": "..." },
  "lakes": [ { "name": "Gatun", "seed": { "name": "Gatun", "lon": -79.9038, "lat": 9.1879 }, "elevation_m": 28, "pixels": 294226 }, ... ],
  "volcanoes": [ { "name": "Barú", "lon": -82.543, "lat": 8.808, "col": 70, "row": 19 }, ... ],
  "stats": { "landforms": { ... }, "depths": { ... }, "flags": { ... }, "sea_distance": [ 0, 1280, 1120, ... ], ... },
  "hexes": [
    { "col": 48, "row": 0, "landform": "cliffs", "surface": "clear", "biome": "clear", "flags": [ "impassable" ], "center": null,
      "pixels": 7982, "valid_pixels": 835, "land_fraction": 0.1046,
      "elevation": { "min": 346, "p5": 366, "median": 423, "p95": 495, "max": 513 }, "relief_m": 129 },
    { "col": 50, "row": 3, "landform": "hills", "flags": [ "river" ], "center": 123,
      "pixels": 7982, "valid_pixels": 7982, "land_fraction": 1,
      "elevation": { "min": 83, "p5": 94, "median": 145, "p95": 258, "max": 369 }, "relief_m": 164 },
    { "col": 36, "row": 7, "landform": "salt-water", "surface": "clear", "biome": "clear", "depth": "shallow", "sea_distance": 1,
      "flags": [ "coast" ], "center": null, "pixels": 7982, "valid_pixels": 942, "land_fraction": 0.1175, ... },
    ...
  ]
}
```

`surface` and `biome` are omitted while unset. `depth` and `sea_distance` are set for salt water only.
`elevation` and `relief_m` are `null` for a hex with no valid pixels.
`stats.sea_distance[d]` is the number of salt-water hexes `d` steps from the nearest hex that isn't salt water.
Hexes are ordered by row, then column.

## Preview

The preview is the `hmz2ele` preview with each hex refilled: land by landform, from pale green (flats) through tan (hills) to brown (mountains), with plateaus orange and volcanic highlands rust; fresh water bright blue; salt water in blues, lightest for coast water and darker with depth; inland seas teal; cliffs red and badlands dark purple; impassable land dark red; volcano hexes magenta.
Rivers are drawn along hex edges in dark blue.

## License

MIT. See `LICENSE`.
