package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"
)

var (
	input        = flag.String("input", "", "Input PNG file with a solder paste map")
	output       = flag.String("output", "", "Output G-code file")
	pxSize       = flag.Float64("px_size", 0, "Size of a pixel side (in mm)")
	toolDiameter = flag.Float64("tool_diameter", 0, "Tool diameter (in mm)")
	millDepth    = flag.Float64("mill_depth", 0, "Mill depth (in mm)")
	safeHeight   = flag.Float64("safe_height", 0, "Safe height to move between mill points (in mm)")
	millRate     = flag.Float64("mill_rate", 0, "Mill rate (mm/min)")
	travelRate   = flag.Float64("travel_rate", 0, "Travel rate (mm/min)")
	n            = flag.Int("n", 1, "Number of linear subpixels for each pixel, when searching for an optimal milling positions")
	background   = flag.String("background", "", "Background color: black or white")

	flagsNotSet []string
)

type Point struct {
	X, Y float64
}

func checkFloat64(name string, val float64) {
	if val == 0 {
		flagsNotSet = append(flagsNotSet, name)
	}
}

func checkString(name string, val string) {
	if val == "" {
		flagsNotSet = append(flagsNotSet, name)
	}
}

func failf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

func main() {
	// Checking flags
	flag.Parse()
	checkString("--input", *input)
	checkString("--output", *output)
	checkString("--background", *background)
	checkFloat64("--px_size", *pxSize)
	checkFloat64("--tool_diameter", *toolDiameter)
	checkFloat64("--mill_depth", *millDepth)
	checkFloat64("--safe_height", *safeHeight)
	checkFloat64("--mill_rate", *millRate)
	checkFloat64("--travel_rate", *travelRate)

	if len(flagsNotSet) > 0 {
		failf("Some mandatory flags not set: %s.\n", strings.Join(flagsNotSet, ", "))
	}

	// Reading input PNG image
	in := mustLoadPNG(*input)

	// Making a gray-scale image with all subpixels. I would prefer to make it a bit image,
	// but image package does not have one, and it's probably unreasonable to implement just
	// for this tiny utility.
	var bk color.Color
	switch *background {
	case "black":
		bk = color.Black
	case "white":
		bk = color.White
	default:
		failf("Unknown color: %s", *background)
	}
	bkr, bkg, bkb, _ := bk.RGBA()

	x0 := in.Bounds().Min.X
	y0 := in.Bounds().Min.Y

	base := image.NewGray(image.Rect(0, 0, in.Bounds().Dx()*(*n), in.Bounds().Dy()*(*n)))
	for i := range base.Pix {
		x := x0 + (i%base.Stride) / *n
		y := y0 + (i/base.Stride) / *n
		cr, cg, cb, _ := in.At(x, y).RGBA()
		if bkr == cr && bkg == cg && bkb == cb {
			base.Pix[i] = 0
		} else {
			base.Pix[i] = 255
		}
	}

	// Save base image for debug purposes
	mustSavePNG("base.debug.png", base)

	// Fill the base image with circles
	// For now, use the dumbest algorithm: triangular tiling with a center in (0,0) and angle = 0
	// See http://en.wikipedia.org/wiki/File:Triangular_tiling_circle_packing.png for the insight
	var best []Point
	shiftN := 4

	try := func(centers []Point) {
		fmt.Printf("Found len(centers) = %d\n", len(centers))
		if len(best) < len(centers) {
			best = centers
		}
		fmt.Printf("len(best) = %d\n", len(best))
	}

	for i := 0; i < shiftN; i++ {
		for j := 0; j < shiftN; j++ {
			try(fillTriangle(base, float64(i)*(*toolDiameter)/2, float64(j)*(*toolDiameter)/2))
			try(fillQuad(base, float64(i)*(*toolDiameter)/2, float64(j)*(*toolDiameter)/2))
		}
	}

	// Create debug output
	basePxSize := *pxSize / float64(*n)
	outImg := image.NewRGBA(base.Bounds())
	draw.Draw(outImg, base.Bounds(), base, image.Point{0, 0}, draw.Src)
	clr := color.RGBA{R: 255, A: 255}
	for _, c := range best {
		drawCircle(outImg, c.X/basePxSize, c.Y/basePxSize, (*toolDiameter)/2/basePxSize, clr)
	}
	mustSavePNG("out.debug.png", outImg)
}

func fillQuad(base *image.Gray, ox, oy float64) []Point {
	basePxSize := *pxSize / float64(*n)
	width := float64(base.Bounds().Dx()) * basePxSize
	height := float64(base.Bounds().Dy()) * basePxSize
	dx := *toolDiameter
	dy := *toolDiameter
	var centers []Point
	for i := 0; ; i++ {
		cx := ox + float64(i)*dx
		if cx >= width {
			break
		}
		for j := 0; ; j++ {
			cy := oy + float64(j)*dy
			if cy >= height {
				break
			}
			if checkCircle(base, basePxSize, cx, cy, (*toolDiameter)/2) {
				centers = append(centers, Point{cx, cy})
			}
		}
	}
	return centers
}

func fillTriangle(base *image.Gray, ox, oy float64) []Point {
	basePxSize := *pxSize / float64(*n)
	width := float64(base.Bounds().Dx()) * basePxSize
	height := float64(base.Bounds().Dy()) * basePxSize

	dy := (*toolDiameter) / 2
	dx := dy * 1.73205080757 // sqrt(3)
	var centers []Point
	for i := 0; ; i++ {
		cx := ox + float64(i)*dx
		if cx >= width {
			break
		}
		for j := 0; ; j++ {
			cy := oy + float64(j)*dy
			if cy >= height {
				break
			}
			if (i+j)%2 == 1 {
				continue
			}
			if checkCircle(base, basePxSize, cx, cy, (*toolDiameter)/2) {
				centers = append(centers, Point{cx, cy})
			}
		}
	}
	return centers
}

func mustLoadPNG(name string) image.Image {
	f, err := os.Open(*input)
	if err != nil {
		failf("Failed to open input file %q: %v", *input, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		failf("Failed to decode a PNG file %q: %v", *input, err)
	}
	return img
}

func mustSavePNG(name string, img image.Image) {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		failf("Failed to create file %q for saving a PNG image: %v", name, err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		failf("Failed to save PNG image to %q: %v", name, err)
	}
}

func drawCircle(img *image.RGBA, x, y, r float64, c color.Color) {
	fmt.Printf("drawCircle(x=%f, y=%f, r=%f, c=%v)\n", x, y, r, c)
	x0 := int(x - r)
	y0 := int(y - r)
	x1 := int(x + r)
	y1 := int(y + r)
	for cy := y0; cy <= y1; cy++ {
		for cx := x0; cx <= x1; cx++ {
			if inside(x, y, r, float64(cx), float64(cy)) {
				img.Set(cx, cy, c)
			}
		}
	}
}

// checkCircle checks that a circle with a center in (x, y) and a radius r fits to the base image and all pixels are high.
func checkCircle(base *image.Gray, pxSize, x, y, r float64) bool {
	width := float64(base.Bounds().Dx()) * pxSize
	height := float64(base.Bounds().Dy()) * pxSize
	if x < r || x > width-r || y < r || y > height-r {
		return false
	}
	x0 := int((x - r) / pxSize)
	y0 := int((y - r) / pxSize)
	x1 := int((x + r) / pxSize)
	y1 := int((y + r) / pxSize)
	for cy := y0; cy <= y1; cy++ {
		i0 := cy * base.Stride
		for cx := x0; cx <= x1; cx++ {
			if !inside(x, y, r, (x-r)+float64(cx-x0)*pxSize, (y-r)+float64(cy-y0)*pxSize) {
				continue
			}
			if base.Pix[i0+cx] == 0 {
				// circle hits background
				//fmt.Printf("checkCircle(pxSize=%f, x=%f, y=%f, r=%f, i0=%d, cx=%d, base.Pix[i0+cx]=%d\n",
				//	pxSize, x, y, r, i0, cx, base.Pix[i0+cx])
				return false
			}
		}
	}
	return true
}

func inside(cx, cy, r, x, y float64) bool {
	return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
}
