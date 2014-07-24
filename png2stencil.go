package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
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
