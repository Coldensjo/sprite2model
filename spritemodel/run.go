package spritemodel

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // sprites also come as GIFs
	"image/png"
)

// Version is the converter's version, shown by the command line and the web page.
const Version = "0.1.0"

// Result is a sprite made into a model: the .glb, the model drawn back the way the sprite was from its
// four sides (a PNG), and what was made. Its JSON (the web page's and the local server's reply) holds
// the bytes as base64.
type Result struct {
	Name   string `json:"name"`
	GLB    []byte `json:"glb"`
	Sides  []byte `json:"sides"`
	Report Report `json:"report"`
	// Summary is the report as a line.
	Summary string `json:"summary"`
	// Frames is how many frames (rows) an outfit sheet holds, 0 for a sprite.
	Frames int `json:"frames"`
}

// Run makes the model of the sprite in data (a PNG or GIF) named name.
func Run(name string, data []byte, opt Options) (*Result, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("not a picture: %w", err)
	}
	name = SafeName(name)
	m, err := Convert(name, img, opt)
	if err != nil {
		return nil, err
	}
	glb, err := m.GLB()
	if err != nil {
		return nil, err
	}
	var sides bytes.Buffer
	if err := png.Encode(&sides, m.Preview(1)); err != nil {
		return nil, err
	}
	r := &Result{Name: name, GLB: glb, Sides: sides.Bytes(), Report: m.Report, Summary: m.Report.Summary()}
	if m.Report.Reading == Outfit {
		b := img.Bounds()
		r.Frames = OutfitFrames(b.Dx(), b.Dy())
	}
	return r, nil
}
