package spritemodel

import (
	"image"
	"image/color"
	"math"
	"sort"
)

// Item reads the sprite as an item drawn flat, the way inventory icons are: a sword, an axe, a bow or
// a shield lying in the picture with its face towards the eye. Its outline is the sprite's, with the
// notches of its staircases filled to the diagonal; how thick it is at each point is guessed from how
// far that point is from the edge and how broad the part it lies in is: a broad part (a blade, an
// axe's head) is flat, bevelled to a thin edge with a ridge down its middle, a narrow one (a grip, a
// guard's arm) round. It is made a smooth mesh rather than voxels, the sprite its texture front and
// back, and stood up: its long axis upright, its blade or head on top, its origin at the grip.
const Item Reading = "item"

// mesh is a model made of triangles rather than cells: three floats a vertex for positions and
// normals, two for texture coordinates, and the texture.
type mesh struct {
	pos, nrm, uvs []float32
	idx           []uint32
	tex           *image.NRGBA
}

// Item thickness: a part's half thickness at its middle is its half width while it is narrow (round),
// a falling share of it as it broadens, down to itemFlat; its edge is itemChunky of that while it is
// narrow, itemSharp once it is broad (a blade's edge).
const (
	itemFlat   = 0.2
	itemChunky = 0.7
	itemSharp  = 0.12
)

// halfThickness is how thick the item is (half of it, either side of its middle plane) at a point d
// from its edge, in a part r broad (the radius of the widest disc inside it that covers the point).
func halfThickness(d, r float64) float64 {
	r = max(r, 0.5)
	share := 1.0
	if r > 1.25 {
		share = max(itemFlat, 1-(r-1.25)*0.5)
	}
	mid := r * share
	edge := mid * min(itemChunky, max(itemSharp, itemChunky-(r-1)*(itemChunky-itemSharp)/1.5))
	t := min(1, d/r)
	// Round across while narrow, bevelled straight to the edge once broad.
	broad := min(1, max(0, (r-1.25)/1.25))
	shape := (1-broad)*math.Sqrt(1-(1-t)*(1-t)) + broad*t
	return edge + (mid-edge)*shape
}

// Quarters of a pixel: the triangles between its centre and each of its sides, top, right, bottom and
// left, in the order of quarterSide. An item covers all four of a drawn pixel, and the two of a clear
// one that lie between two drawn neighbours at right angles: the notch of a staircase is filled to
// the diagonal, so a line drawn at 45 degrees has a straight edge, and a one-pixel line drawn
// diagonally is joined up rather than a string of beads.
var quarterSide = [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

// quarterCorner is the corner each quarter starts from, clockwise in the picture, in half pixels.
var quarterCorner = [4][2]int{{0, 0}, {2, 0}, {2, 2}, {0, 2}}

// itemCover is which quarters of each pixel the item covers, a bit a quarter.
func (s *sprite) itemCover() []uint8 {
	cover := make([]uint8, s.w*s.h)
	for py := range s.h {
		for px := range s.w {
			if s.isOpaque(px, py) {
				cover[py*s.w+px] = 0b1111
				continue
			}
			for k := range 4 {
				a, b := quarterSide[k], quarterSide[(k+1)%4]
				if s.isOpaque(px+a[0], py+a[1]) && s.isOpaque(px+b[0], py+b[1]) {
					cover[py*s.w+px] |= 1<<k | 1<<((k+1)%4)
				}
			}
		}
	}
	return cover
}

// covers says whether the item covers quarter k of the pixel (px, py).
func (s *sprite) covers(cover []uint8, px, py, k int) bool {
	return px >= 0 && py >= 0 && px < s.w && py < s.h && cover[py*s.w+px]&(1<<k) != 0
}

// edgeDistance is how far the point (fx, fy), in pixels, is from the nearest part of the picture the
// item does not cover (outside the sprite included), searching out a ring of pixels at a time.
func (s *sprite) edgeDistance(cover []uint8, fx, fy float64) float64 {
	bx, by := int(math.Floor(fx)), int(math.Floor(fy))
	best := math.Inf(1)
	p := [2]float64{fx, fy}
	try := func(x, y int) {
		c := uint8(0)
		if x >= 0 && y >= 0 && x < s.w && y < s.h {
			c = cover[y*s.w+x]
		}
		switch c {
		case 0b1111:
			return
		case 0:
			dx := math.Max(0, math.Max(float64(x)-fx, fx-float64(x+1)))
			dy := math.Max(0, math.Max(float64(y)-fy, fy-float64(y+1)))
			best = min(best, math.Hypot(dx, dy))
			return
		}
		mid := [2]float64{float64(x) + 0.5, float64(y) + 0.5}
		for k := range 4 {
			if c&(1<<k) != 0 {
				continue
			}
			a, b := quarterCorner[k], quarterCorner[(k+1)%4]
			best = min(best, triangleDistance(p, mid,
				[2]float64{float64(x) + float64(a[0])/2, float64(y) + float64(a[1])/2},
				[2]float64{float64(x) + float64(b[0])/2, float64(y) + float64(b[1])/2}))
		}
	}
	try(bx, by)
	for r := 1; float64(r-1) < best; r++ {
		for x := bx - r; x <= bx+r; x++ {
			try(x, by-r)
			try(x, by+r)
		}
		for y := by - r + 1; y < by+r; y++ {
			try(bx-r, y)
			try(bx+r, y)
		}
	}
	return best
}

// triangleDistance is how far p is from the triangle abc: 0 inside it.
func triangleDistance(p, a, b, c [2]float64) float64 {
	side := func(u, v [2]float64) float64 { return (v[0]-u[0])*(p[1]-u[1]) - (v[1]-u[1])*(p[0]-u[0]) }
	s1, s2, s3 := side(a, b), side(b, c), side(c, a)
	if (s1 >= 0 && s2 >= 0 && s3 >= 0) || (s1 <= 0 && s2 <= 0 && s3 <= 0) {
		return 0
	}
	seg := func(u, v [2]float64) float64 {
		dx, dy := v[0]-u[0], v[1]-u[1]
		t := min(1, max(0, ((p[0]-u[0])*dx+(p[1]-u[1])*dy)/(dx*dx+dy*dy)))
		return math.Hypot(p[0]-u[0]-t*dx, p[1]-u[1]-t*dy)
	}
	return min(seg(a, b), seg(b, c), seg(c, a))
}

// item makes the model of the sprite read as an item drawn flat (Item). unit is a pixel's size in the
// model; grip, when given, is where its origin goes, in sprite pixels.
func (s *sprite) item(m *Model, unit float64, grip *image.Point) {
	w, h := s.w, s.h
	cover := s.itemCover()
	// The lattice of pixel corners and centres, at half pixels: point (i, j) is at (i/2, j/2).
	lw, lh := 2*w+1, 2*h+1
	dist := make([]float64, lw*lh)
	for j := range lh {
		for i := range lw {
			dist[j*lw+i] = s.edgeDistance(cover, float64(i)/2, float64(j)/2)
		}
	}
	// How broad the part about each point is: the widest disc inside the item that covers it, from the
	// discs about the ridge (points no neighbour is further from the edge than).
	broad := make([]float64, lw*lh)
	copy(broad, dist)
	for j := range lh {
		for i := range lw {
			d := dist[j*lw+i]
			if d <= 0 || !ridge(dist, lw, lh, i, j) {
				continue
			}
			reach := d + 0.5 // a pixel's corner is still covered by the disc of a one-pixel line
			k := int(math.Ceil(2 * reach))
			for y := max(0, j-k); y <= min(lh-1, j+k); y++ {
				for x := max(0, i-k); x <= min(lw-1, i+k); x++ {
					if math.Hypot(float64(x-i), float64(y-j))/2 <= reach {
						broad[y*lw+x] = max(broad[y*lw+x], d)
					}
				}
			}
		}
	}
	half := func(i, j int) float64 { return halfThickness(dist[j*lw+i], broad[j*lw+i]) }

	rot, origin := s.itemPose(grip)
	cosR, sinR := math.Cos(rot), math.Sin(rot)
	// place is a point of the picture at depth z (pixels, towards the eye) in the model: x right, y up,
	// z towards the eye, turned upright about the grip.
	place := func(fx, fy, z float64) [3]float32 {
		x, y := fx-origin[0], origin[1]-fy
		return [3]float32{float32((x*cosR - y*sinR) * unit), float32((x*sinR + y*cosR) * unit), float32(z * unit)}
	}
	turn := func(n [3]float64) [3]float64 {
		return [3]float64{n[0]*cosR - n[1]*sinR, n[0]*sinR + n[1]*cosR, n[2]}
	}

	tw, th := pow2(w), pow2(h)
	me := &mesh{tex: bleed(s.pix, cover, tw, th)}
	// tri adds a triangle with flat shading, wound counter-clockwise about the side facing out (out, in
	// the picture's axes).
	tri := func(p [3][3]float64, uv [3][2]float64, out [3]float64) {
		var q [3][3]float32
		for k := range 3 {
			q[k] = place(p[k][0], p[k][1], p[k][2])
		}
		// In the model the picture's y runs up, so the picture's axes are mirrored: work in the model's.
		e1 := [3]float64{float64(q[1][0] - q[0][0]), float64(q[1][1] - q[0][1]), float64(q[1][2] - q[0][2])}
		e2 := [3]float64{float64(q[2][0] - q[0][0]), float64(q[2][1] - q[0][1]), float64(q[2][2] - q[0][2])}
		n := [3]float64{e1[1]*e2[2] - e1[2]*e2[1], e1[2]*e2[0] - e1[0]*e2[2], e1[0]*e2[1] - e1[1]*e2[0]}
		o := turn([3]float64{out[0], -out[1], out[2]})
		order := [3]int{0, 1, 2}
		if n[0]*o[0]+n[1]*o[1]+n[2]*o[2] < 0 {
			order = [3]int{0, 2, 1}
			n = [3]float64{-n[0], -n[1], -n[2]}
		}
		l := math.Sqrt(n[0]*n[0] + n[1]*n[1] + n[2]*n[2])
		if l == 0 {
			return
		}
		base := uint32(len(me.pos) / 3)
		for _, k := range order {
			me.pos = append(me.pos, q[k][:]...)
			me.nrm = append(me.nrm, float32(n[0]/l), float32(n[1]/l), float32(n[2]/l))
			me.uvs = append(me.uvs, float32(uv[k][0]/float64(tw)), float32(uv[k][1]/float64(th)))
		}
		me.idx = append(me.idx, base, base+1, base+2)
	}
	// wall adds the rim between the points u and v (half thicknesses hu, hv), facing out, coloured c.
	wall := func(u, v [2]float64, hu, hv float64, out [2]int, c [2]float64) {
		uv := [3][2]float64{c, c, c}
		o := [3]float64{float64(out[0]), float64(out[1]), 0}
		tri([3][3]float64{{u[0], u[1], hu}, {v[0], v[1], hv}, {v[0], v[1], -hv}}, uv, o)
		tri([3][3]float64{{u[0], u[1], hu}, {v[0], v[1], -hv}, {u[0], u[1], -hu}}, uv, o)
	}

	at := func(i, j int) [2]float64 { return [2]float64{float64(i) / 2, float64(j) / 2} }
	for py := range h {
		for px := range w {
			if cover[py*w+px] == 0 {
				continue
			}
			ci, cj := 2*px+1, 2*py+1
			c, hc := at(ci, cj), half(ci, cj)
			for k := range 4 {
				if !s.covers(cover, px, py, k) {
					continue
				}
				// Front and back: the quarter's triangle, from the pixel's centre to one of its sides.
				a, b := quarterCorner[k], quarterCorner[(k+1)%4]
				ai, aj, bi, bj := 2*px+a[0], 2*py+a[1], 2*px+b[0], 2*py+b[1]
				pa, pb := at(ai, aj), at(bi, bj)
				ha, hb := half(ai, aj), half(bi, bj)
				for _, side := range [2]float64{1, -1} {
					tri([3][3]float64{{c[0], c[1], side * hc}, {pa[0], pa[1], side * ha}, {pb[0], pb[1], side * hb}},
						[3][2]float64{c, pa, pb}, [3]float64{0, 0, side})
				}
				// The rim, where the quarter meets one not covered, in the pixel's colour: across its side,
				// or within the pixel towards the quarter before or after it.
				if d := quarterSide[k]; !s.covers(cover, px+d[0], py+d[1], (k+2)%4) {
					wall(pa, pb, ha, hb, d, c)
				}
				if prev := (k + 3) % 4; !s.covers(cover, px, py, prev) {
					wall(c, pa, hc, ha, quarterSide[prev], c)
				}
				if next := (k + 1) % 4; !s.covers(cover, px, py, next) {
					wall(c, pb, hc, hb, quarterSide[next], c)
				}
			}
		}
	}

	m.mesh = me
	lo := [3]float32{math.MaxFloat32, math.MaxFloat32, math.MaxFloat32}
	hi := [3]float32{-math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}
	for i := 0; i < len(me.pos); i += 3 {
		for a := range 3 {
			lo[a], hi[a] = min(lo[a], me.pos[i+a]), max(hi[a], me.pos[i+a])
		}
	}
	for a := range 3 {
		m.Report.Size[a] = float64(hi[a] - lo[a]) // width, height, depth
	}
	m.Report.Reading = Item
	m.Report.Covered = 100
	m.Report.Centre = image.Pt(int(math.Floor(origin[0])), int(math.Floor(origin[1])))
	m.Report.Triangles = len(me.idx) / 3
}

// ridge says whether no neighbour of the lattice point is further from the edge than it.
func ridge(dist []float64, lw, lh, i, j int) bool {
	d := dist[j*lw+i]
	for y := max(0, j-1); y <= min(lh-1, j+1); y++ {
		for x := max(0, i-1); x <= min(lw-1, i+1); x++ {
			if dist[y*lw+x] > d {
				return false
			}
		}
	}
	return true
}

// spread is the centre of the sprite's drawn pixels and how they spread about it: its two principal
// variances (l1 the larger) and the angle of the axis along which they spread most, in the picture.
func (s *sprite) spread() (mx, my, l1, l2, theta float64) {
	var n float64
	for i, o := range s.opaque {
		if o {
			n, mx, my = n+1, mx+float64(i%s.w)+0.5, my+float64(i/s.w)+0.5
		}
	}
	mx, my = mx/n, my/n
	var sxx, syy, sxy float64
	for i, o := range s.opaque {
		if o {
			dx, dy := float64(i%s.w)+0.5-mx, float64(i/s.w)+0.5-my
			sxx, syy, sxy = sxx+dx*dx, syy+dy*dy, sxy+dx*dy
		}
	}
	sxx, syy, sxy = sxx/n, syy/n, sxy/n
	tr, det := sxx+syy, sxx*syy-sxy*sxy
	l1 = tr/2 + math.Sqrt(math.Max(0, tr*tr/4-det))
	return mx, my, l1, tr - l1, 0.5 * math.Atan2(2*sxy, sxx-syy)
}

// itemLong is how many times longer than broad (in spread) a thing must be to be stood up as an
// item, and for Auto to take a sprite it cannot stand on its tile for an item.
const itemLong = 3.0

// elongation is how many times the sprite's spread along its longest axis is its spread across.
func (s *sprite) elongation() float64 {
	_, _, l1, l2, _ := s.spread()
	if l2 <= 0 {
		return math.Inf(1)
	}
	return math.Sqrt(l1 / l2)
}

// itemPose finds how to stand the item up: the turn (radians, counter-clockwise in the model) that
// stands its long axis straight up, blade or head on top, and its grip in pixels, the model's origin:
// the middle of the handle below a guard where one sticks out, else a fifth of the way up from its
// foot; the centre of something not long. A grip given is kept.
func (s *sprite) itemPose(grip *image.Point) (rot float64, origin [2]float64) {
	mx, my, l1, l2, theta := s.spread()
	origin = [2]float64{mx, my}
	if grip != nil {
		origin = [2]float64{float64(grip.X) + 0.5, float64(grip.Y) + 0.5}
	}
	if l2 > 0 && l1/l2 < itemLong*itemLong {
		return 0, origin // not long: left as drawn
	}
	// Icons are drawn straight or at 45 degrees: snap to that when near it.
	if snap := math.Round(theta/(math.Pi/4)) * math.Pi / 4; math.Abs(theta-snap) < 10*math.Pi/180 {
		theta = snap
	}
	axis := [2]float64{math.Cos(theta), math.Sin(theta)}
	along := func(i int) (a, c float64) {
		dx, dy := float64(i%s.w)+0.5-mx, float64(i/s.w)+0.5-my
		return dx*axis[0] + dy*axis[1], dy*axis[0] - dx*axis[1]
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for i, o := range s.opaque {
		if o {
			a, _ := along(i)
			lo, hi = min(lo, a), max(hi, a)
		}
	}
	// Point the axis at the business end: the one with more bare metal (grey, not dark: a blade, an
	// axe's head) when the halves clearly differ, else the heavier, which the mass sits nearer.
	var grey [2]float64
	for i, o := range s.opaque {
		if !o {
			continue
		}
		c := s.pix.NRGBAAt(i%s.w, i/s.w)
		top, bottom := max(c.R, c.G, c.B), min(c.R, c.G, c.B)
		if int(top)-int(bottom) < 40 && top > 70 {
			if a, _ := along(i); a > (lo+hi)/2 {
				grey[1]++
			} else {
				grey[0]++
			}
		}
	}
	flip := (lo+hi)/2 > 0
	if grey[0]+grey[1] >= 4 && (grey[0] > 1.5*grey[1] || grey[1] > 1.5*grey[0]) {
		flip = grey[0] > grey[1]
	}
	if flip {
		axis = [2]float64{-axis[0], -axis[1]}
		lo, hi = -hi, -lo
	}
	// The model's y is the picture's -y: the axis in the model's axes is (ax, -ay); turn it to (0, 1).
	rot = math.Pi/2 - math.Atan2(-axis[1], axis[0])
	if grip != nil {
		return rot, origin
	}
	// Along the axis, a pixel's step at a time from the foot: how far each reaches across it.
	steps := int(hi-lo) + 1
	across := make([][2]float64, steps)
	for k := range across {
		across[k] = [2]float64{math.Inf(1), math.Inf(-1)}
	}
	for i, o := range s.opaque {
		if o {
			a, c := along(i)
			k := min(steps-1, int(a-lo))
			across[k] = [2]float64{min(across[k][0], c), max(across[k][1], c)}
		}
	}
	width := func(k int) float64 { return max(0, across[k][1]-across[k][0]) }
	// A guard: in the lower part, a step clearly wider than the handle below it.
	at := 0.2 * (hi - lo)
	guard, widest := 0, 0.0
	for k := 2; float64(k) <= 0.45*(hi-lo) && k < steps; k++ {
		if wd := width(k); wd > widest {
			widest, guard = wd, k
		}
	}
	if guard > 0 {
		var below []float64
		for k := range guard {
			below = append(below, width(k))
		}
		sort.Float64s(below)
		if widest >= 1.8*below[len(below)/2] {
			at = float64(guard) / 2
		}
	}
	k := min(steps-1, int(at))
	mid := 0.0
	if !math.IsInf(across[k][0], 1) {
		mid = (across[k][0] + across[k][1]) / 2
	}
	a := lo + at
	origin = [2]float64{mx + a*axis[0] - mid*axis[1], my + a*axis[1] + mid*axis[0]}
	return rot, origin
}

// bleed is the sprite as a texture tw x th, every pixel opaque: a clear pixel the item covers part of
// takes the colour of the drawn pixels beside it, the others that of the nearest drawn pixel, so
// sampling at a pixel's edge never picks up a clear neighbour.
func bleed(pix *image.NRGBA, cover []uint8, tw, th int) *image.NRGBA {
	w, h := pix.Rect.Dx(), pix.Rect.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, tw, th))
	for y := range th {
		for x := range tw {
			px, py := min(x, w-1), min(y, h-1)
			c := pix.NRGBAAt(px, py)
			if c.A < 128 {
				var r, g, b, n int
				if cover[py*w+px] != 0 {
					for _, d := range quarterSide {
						nx, ny := px+d[0], py+d[1]
						if nx >= 0 && ny >= 0 && nx < w && ny < h {
							if o := pix.NRGBAAt(nx, ny); o.A >= 128 {
								r, g, b, n = r+int(o.R), g+int(o.G), b+int(o.B), n+1
							}
						}
					}
				}
				if n > 0 {
					c = color.NRGBA{uint8(r / n), uint8(g / n), uint8(b / n), 255}
				} else {
					c = nearestOpaque(pix, px, py)
				}
			}
			c.A = 255
			out.SetNRGBA(x, y, c)
		}
	}
	return out
}

// preview draws the mesh turned about its upright axis to face front, three-quarters, edge on and
// back, side by side, lit from the upper left: each sprite pixel ppx pixels.
func (me *mesh) preview(unit float64, ppx int) *image.NRGBA {
	var rad, ylo, yhi float64 = 0, math.Inf(1), math.Inf(-1)
	for i := 0; i < len(me.pos); i += 3 {
		x, y, z := float64(me.pos[i]), float64(me.pos[i+1]), float64(me.pos[i+2])
		rad = max(rad, math.Hypot(x, z))
		ylo, yhi = min(ylo, y), max(yhi, y)
	}
	k := float64(ppx) / unit // preview pixels a model unit
	pw := int(math.Ceil(2*rad*k)) + 2*ppx
	ph := int(math.Ceil((yhi-ylo)*k)) + 2*ppx
	gap := 4 * ppx
	yaws := [4]float64{0, math.Pi / 4, math.Pi / 2, math.Pi}
	out := image.NewNRGBA(image.Rect(0, 0, 4*pw+3*gap, ph))
	for i := 0; i < len(out.Pix); i += 4 {
		copy(out.Pix[i:], []byte{0x30, 0x30, 0x34, 0xff})
	}
	light := [3]float64{-0.45, 0.6, 0.66}
	tw, th := me.tex.Rect.Dx(), me.tex.Rect.Dy()
	depth := make([]float64, pw*ph)
	for v, yaw := range yaws {
		for i := range depth {
			depth[i] = math.Inf(-1)
		}
		c, sn := math.Cos(yaw), math.Sin(yaw)
		ox := v * (pw + gap)
		for t := 0; t < len(me.idx); t += 3 {
			var sx, sy, sz [3]float64
			var uv [3][2]float64
			for j := range 3 {
				q := me.idx[t+j]
				x, y, z := float64(me.pos[3*q]), float64(me.pos[3*q+1]), float64(me.pos[3*q+2])
				sx[j] = (x*c-z*sn)*k + float64(pw)/2
				sy[j] = (yhi-y)*k + float64(ppx)
				sz[j] = x*sn + z*c
				uv[j] = [2]float64{float64(me.uvs[2*q]) * float64(tw), float64(me.uvs[2*q+1]) * float64(th)}
			}
			q := me.idx[t]
			nx, ny, nz := float64(me.nrm[3*q]), float64(me.nrm[3*q+1]), float64(me.nrm[3*q+2])
			nx, nz = nx*c-nz*sn, nx*sn+nz*c
			if nz < 0 {
				continue // facing away
			}
			shade := 0.45 + 0.55*math.Max(0, nx*light[0]+ny*light[1]+nz*light[2])
			area := (sx[1]-sx[0])*(sy[2]-sy[0]) - (sx[2]-sx[0])*(sy[1]-sy[0])
			if math.Abs(area) < 1e-9 {
				continue
			}
			x0, x1 := int(math.Floor(min(sx[0], sx[1], sx[2]))), int(math.Ceil(max(sx[0], sx[1], sx[2])))
			y0, y1 := int(math.Floor(min(sy[0], sy[1], sy[2]))), int(math.Ceil(max(sy[0], sy[1], sy[2])))
			for py := max(0, y0); py < min(ph, y1+1); py++ {
				for px := max(0, x0); px < min(pw, x1+1); px++ {
					fx, fy := float64(px)+0.5, float64(py)+0.5
					b0 := ((sx[1]-fx)*(sy[2]-fy) - (sx[2]-fx)*(sy[1]-fy)) / area
					b1 := ((sx[2]-fx)*(sy[0]-fy) - (sx[0]-fx)*(sy[2]-fy)) / area
					b2 := 1 - b0 - b1
					if b0 < 0 || b1 < 0 || b2 < 0 {
						continue
					}
					z := b0*sz[0] + b1*sz[1] + b2*sz[2]
					if z <= depth[py*pw+px] {
						continue
					}
					depth[py*pw+px] = z
					u := b0*uv[0][0] + b1*uv[1][0] + b2*uv[2][0]
					w := b0*uv[0][1] + b1*uv[1][1] + b2*uv[2][1]
					col := me.tex.NRGBAAt(min(max(int(u), 0), tw-1), min(max(int(w), 0), th-1))
					out.SetNRGBA(ox+px, py, color.NRGBA{
						uint8(min(255, float64(col.R)*shade)), uint8(min(255, float64(col.G)*shade)), uint8(min(255, float64(col.B)*shade)), 255,
					})
				}
			}
		}
	}
	return out
}
