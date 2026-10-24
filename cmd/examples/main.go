// Command examples draws the page's example sprites (web/examples) from small voxel scenes, in the
// projection the converter reads (a point at tile position (x, y) and height z drawn at (x - z, y - z),
// 32 pixels a tile), shaded and outlined the way such sprites are: originals, so the repository holds
// no one else's art.
//
//	go run ./cmd/examples [-out web/examples]
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// scene is a set of coloured cells, keyed by position.
type scene map[[3]int]color.NRGBA

func (s scene) box(x0, y0, z0, x1, y1, z1 int, c func(x, y, z int) color.NRGBA) {
	for z := z0; z < z1; z++ {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				s[[3]int{x, y, z}] = c(x, y, z)
			}
		}
	}
}

func (s scene) cylinder(cx, cy float64, r float64, z0, z1 int, c func(x, y, z int) color.NRGBA) {
	for z := z0; z < z1; z++ {
		for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
			for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
				if math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) <= r {
					s[[3]int{x, y, z}] = c(x, y, z)
				}
			}
		}
	}
}

// jitter varies a colour a little by position, as hand-drawn texture does.
func jitter(c color.NRGBA, x, y, z int, amount float64) color.NRGBA {
	h := uint32(x*73856093) ^ uint32(y*19349663) ^ uint32(z*83492791)
	h ^= h >> 13
	h *= 0x5bd1e995
	f := 1 + amount*(float64(h%1000)/1000-0.5)*2
	cl := func(v uint8) uint8 { return uint8(math.Max(0, math.Min(255, float64(v)*f))) }
	return color.NRGBA{cl(c.R), cl(c.G), cl(c.B), 255}
}

func shade(c color.NRGBA, f float64) color.NRGBA {
	cl := func(v uint8) uint8 { return uint8(math.Max(0, math.Min(255, float64(v)*f))) }
	return color.NRGBA{cl(c.R), cl(c.G), cl(c.B), 255}
}

// draw renders the scene into a w x h sprite, the ground point (ox, oy) of cell (0, 0): a cell's south
// or east side at its pixel, its top at the pixel up and to the left, nearer things over farther, the
// light from the north-west; then a dark outline round the silhouette.
func draw(s scene, w, h, ox, oy int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	depth := make([]int, w*h)
	for i := range depth {
		depth[i] = -1 << 30
	}
	plot := func(px, py, d int, c color.NRGBA) {
		if px < 0 || py < 0 || px >= w || py >= h || d <= depth[py*w+px] {
			return
		}
		depth[py*w+px] = d
		img.SetNRGBA(px, py, c)
	}
	for p, c := range s {
		x, y, z := p[0], p[1], p[2]
		_, south := s[[3]int{x, y + 1, z}]
		_, east := s[[3]int{x + 1, y, z}]
		_, top := s[[3]int{x, y, z + 1}]
		px, py := x-z+ox, y-z+oy
		switch {
		case !south:
			plot(px, py, 2*z, shade(c, 0.78))
		case !east:
			plot(px, py, 2*z, shade(c, 0.62))
		case !top:
			plot(px, py, 2*z, c)
		}
		if !top {
			plot(px-1, py-1, 2*z+1, c)
		}
		if !south {
			plot(px-1, py, 2*z-1, shade(c, 0.78))
		}
		if !east {
			plot(px, py-1, 2*z-1, shade(c, 0.62))
		}
	}
	// The outline: clear pixels touching the drawing.
	out := image.NewNRGBA(img.Rect)
	copy(out.Pix, img.Pix)
	for y := range h {
		for x := range w {
			if img.NRGBAAt(x, y).A != 0 {
				continue
			}
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if nx >= 0 && ny >= 0 && nx < w && ny < h && img.NRGBAAt(nx, ny).A != 0 {
					out.SetNRGBA(x, y, color.NRGBA{12, 10, 10, 255})
					break
				}
			}
		}
	}
	return out
}

var (
	stone = color.NRGBA{142, 140, 146, 255}
	dark  = color.NRGBA{96, 94, 102, 255}
	wood  = color.NRGBA{150, 98, 52, 255}
	plank = color.NRGBA{112, 70, 36, 255}
	iron  = color.NRGBA{70, 72, 80, 255}
	brick = color.NRGBA{150, 62, 46, 255}
	mortr = color.NRGBA{120, 112, 100, 255}
	leaf  = color.NRGBA{58, 120, 52, 255}
	bark  = color.NRGBA{96, 64, 40, 255}
	clay  = color.NRGBA{120, 132, 150, 255}
	glow  = color.NRGBA{255, 214, 60, 255}
)

func pillar() scene {
	s := scene{}
	st := func(x, y, z int) color.NRGBA { return jitter(stone, x, y, z, 0.08) }
	s.box(-13, -13, 0, 13, 13, 2, st)
	s.box(-8, -8, 2, 8, 8, 27, func(x, y, z int) color.NRGBA {
		if (z-2)%8 == 7 {
			return jitter(dark, x, y, z, 0.05)
		}
		return st(x, y, z)
	})
	s.box(-10, -10, 27, 10, 10, 29, st)
	s.box(-13, -13, 29, 13, 13, 32, st)
	return s
}

func crate() scene {
	s := scene{}
	s.box(-12, -12, 0, 12, 12, 22, func(x, y, z int) color.NRGBA {
		edge := 0
		for _, v := range [3]int{x + 12, y + 12, z} {
			if v < 2 || v >= map[bool]int{true: 22, false: 24}[v == z]-2 {
				edge++
			}
		}
		if edge >= 2 {
			return jitter(plank, x, y, z, 0.06)
		}
		if (z/5)%2 == 0 {
			return jitter(wood, x, y, z, 0.1)
		}
		return jitter(shade(wood, 0.9), x, y, z, 0.1)
	})
	return s
}

func barrel() scene {
	s := scene{}
	for z := range 24 {
		r := 9 + 1.5*math.Sin(math.Pi*float64(z)/23)
		s.cylinder(0, 0, r, z, z+1, func(x, y, z int) color.NRGBA {
			if z == 3 || z == 4 || z == 19 || z == 20 {
				return jitter(iron, x, y, z, 0.05)
			}
			return jitter(wood, x, y, z, 0.12)
		})
	}
	return s
}

func tree() scene {
	s := scene{}
	s.cylinder(0, 0, 3, 0, 10, func(x, y, z int) color.NRGBA { return jitter(bark, x, y, z, 0.1) })
	for z := 8; z < 44; z++ {
		t := float64(z-8) / 36
		r := 13*(1-t) + 1
		if (z-8)%9 > 6 {
			r -= 2 // the layers of a fir
		}
		s.cylinder(0, 0, r, z, z+1, func(x, y, z int) color.NRGBA { return jitter(leaf, x, y, z, 0.25) })
	}
	return s
}

func wall() scene {
	s := scene{}
	s.box(-16, 8, 0, 16, 14, 32, func(x, y, z int) color.NRGBA {
		row := z / 4
		off := 0
		if row%2 == 1 {
			off = 4
		}
		if z%4 == 3 || (x+16+off)%8 == 7 {
			return jitter(mortr, x, y, z, 0.06)
		}
		return jitter(brick, x, y, z, 0.12)
	})
	return s
}

// golem is a small figure facing south (+y), its eyes on its face, small enough for a 32 pixel frame
// from every side: an outfit to draw four ways.
func golem() scene {
	s := scene{}
	c := func(x, y, z int) color.NRGBA { return jitter(clay, x, y, z, 0.12) }
	s.box(-4, -1, 0, -1, 2, 6, c) // legs
	s.box(1, -1, 0, 4, 2, 6, c)
	s.box(-4, -2, 6, 4, 2, 13, c)  // body
	s.box(-6, -1, 7, -4, 1, 13, c) // arms
	s.box(4, -1, 7, 6, 1, 13, c)
	s.box(-3, -2, 13, 3, 2, 18, c) // head
	s[[3]int{-2, 1, 15}] = glow    // eyes, on the south face
	s[[3]int{1, 1, 15}] = glow
	return s
}

// turned is the scene turned so it faces dir (0 north, 1 east, 2 south, 3 west), about the axis.
func turned(s scene, dir int) scene {
	out := scene{}
	for p, c := range s {
		a, b := p[0], p[1]
		switch dir {
		case 0:
			a, b = -a-1, -b-1
		case 1:
			a, b = b, -a-1
		case 3:
			a, b = -b-1, a
		}
		out[[3]int{a, b, p[2]}] = c
	}
	return out
}

func main() {
	dir := flag.String("out", "web/examples", "where to write the sprites")
	flag.Parse()
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	save := func(name string, img image.Image) {
		f, err := os.Create(filepath.Join(*dir, name+".png"))
		if err == nil {
			err = png.Encode(f, img)
			f.Close()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	// A tile's centre is (48, 48) in a 64 x 64 sprite: its bottom-right tile.
	save("pillar", draw(pillar(), 64, 64, 48, 48))
	save("crate", draw(crate(), 64, 64, 48, 48))
	save("barrel", draw(barrel(), 64, 64, 48, 48))
	save("tree", draw(tree(), 64, 64, 48, 48))
	save("wall", draw(wall(), 64, 64, 48, 48))
	// The outfit: four 32 x 32 frames facing north, east, south and west.
	sheet := image.NewNRGBA(image.Rect(0, 0, 128, 32))
	for d := range 4 {
		f := draw(turned(golem(), d), 32, 32, 24, 25)
		for y := range 32 {
			for x := range 32 {
				sheet.SetNRGBA(d*32+x, y, f.NRGBAAt(x, y))
			}
		}
	}
	save("golem_outfit", sheet)
}
