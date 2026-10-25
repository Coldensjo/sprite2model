// Command sprite2model turns sprites drawn in a slanted projection (Tibia-style item sprites, 2:1
// isometric sprites, a creature's four-way outfit sheet) or an item's icon drawn flat into 3D models,
// glTF binaries (.glb):
//
//	sprite2model [flags] <sprite.png>...   convert, writing <name>.glb beside each sprite (or into -out)
//	sprite2model serve [-addr host:port]   open the converter's page in the browser, run locally
//
// Flags:
//
//	-proj tibia|iso     how the sprite is drawn (default tibia)
//	-reading auto|box|round|wall|cutout|hull|outfit|item
//	                    how to guess its depth (default auto)
//	-outline            keep the dark outline round the silhouette in the colours
//	-centre x,y         where its axis meets the ground (an item's grip), in sprite pixels (default: found)
//	-scale f            size factor; 1 makes one tile one unit (default 1)
//	-frame n            for an outfit sheet, the row (a frame of its walk) to read, from 0
//	-out dir            folder for the models (default: beside each sprite)
//	-preview            also write <name>.preview.png, the model drawn from its four sides
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/Coldensjo/sprite2model/spritemodel"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		serve(os.Args[2:])
		return
	}
	proj := flag.String("proj", "tibia", "how the sprite is drawn: tibia, or iso (2:1 isometric)")
	reading := flag.String("reading", "auto", "how to guess the depth: auto, box, round, wall, cutout, hull, outfit (a sheet facing four ways), or item (a weapon or tool drawn flat)")
	outline := flag.Bool("outline", false, "keep the dark outline round the silhouette in the colours")
	centre := flag.String("centre", "", "where the thing's axis meets the ground (an item's grip), as x,y in sprite pixels (default: found)")
	scale := flag.Float64("scale", 1, "size factor; 1 makes one tile one unit")
	frame := flag.Int("frame", 0, "for an outfit sheet, the row (frame of its walk) to read, from 0")
	outDir := flag.String("out", "", "folder for the models (default: beside each sprite)")
	preview := flag.Bool("preview", false, "also write <name>.preview.png, the model drawn from its four sides")
	version := flag.Bool("version", false, "print the version")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "sprite2model %s: sprites drawn slanted made into 3D models (.glb)\n\n", spritemodel.Version)
		fmt.Fprintln(os.Stderr, "usage: sprite2model [flags] <sprite.png>...")
		fmt.Fprintln(os.Stderr, "       sprite2model serve [-addr host:port]   the converter's page, in the browser")
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
	}
	flag.Parse()
	if *version {
		fmt.Println(spritemodel.Version)
		return
	}
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	opt := spritemodel.Options{
		Projection:  spritemodel.Projection(*proj),
		Reading:     spritemodel.Reading(*reading),
		KeepOutline: *outline,
		Scale:       *scale,
		Frame:       *frame,
	}
	if *centre != "" {
		var p image.Point
		if _, err := fmt.Sscanf(*centre, "%d,%d", &p.X, &p.Y); err != nil {
			fmt.Fprintf(os.Stderr, "-centre %q: want x,y\n", *centre)
			os.Exit(2)
		}
		opt.Centre = &p
	}
	failed := false
	for _, in := range flag.Args() {
		if err := convert(in, *outDir, opt, *preview); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", in, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func convert(in, outDir string, opt spritemodel.Options, preview bool) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
	r, err := spritemodel.Run(base, data, opt)
	if err != nil {
		return err
	}
	dir := outDir
	if dir == "" {
		dir = filepath.Dir(in)
	}
	out := filepath.Join(dir, r.Name+".glb")
	if err := os.WriteFile(out, r.GLB, 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: %s\n", out, r.Summary)
	if preview {
		return os.WriteFile(strings.TrimSuffix(out, ".glb")+".preview.png", r.Sides, 0o644)
	}
	return nil
}
