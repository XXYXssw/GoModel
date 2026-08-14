// Command genicon renders the GoModel hexagon mark as PNG icons: a black
// template icon for the menu bar (internal/tray/assets/icon.png) and a
// brand-colored iconset for the macOS app bundle (build/GoModel.iconset).
// The macOS .icns is produced from the iconset with `iconutil` (see `make app`).
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
)

const brand = "#755c3d"

func main() {
	out := "."
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	// Menu bar template icon: black + alpha, 32x32.
	if err := writePNG(filepath.Join(out, "icon.png"), render(32, color.RGBA{0, 0, 0, 255})); err != nil {
		fatal(err)
	}
	// App iconset sizes (points and @2x pixels).
	sizes := []struct {
		name string
		px   int
	}{
		{"icon_16x16.png", 16}, {"icon_16x16@2x.png", 32},
		{"icon_32x32.png", 32}, {"icon_32x32@2x.png", 64},
		{"icon_128x128.png", 128}, {"icon_128x128@2x.png", 256},
		{"icon_256x256.png", 256}, {"icon_256x256@2x.png", 512},
		{"icon_512x512.png", 512}, {"icon_512x512@2x.png", 1024},
	}
	brandRGBA, err := hexToRGBA(brand)
	if err != nil {
		fatal(err)
	}
	for _, s := range sizes {
		if err := writePNG(filepath.Join(out, s.name), render(s.px, brandRGBA)); err != nil {
			fatal(err)
		}
	}
}

func hexToRGBA(hex string) (color.RGBA, error) {
	v, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("bad hex color %q: %w", hex, err)
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
}

// render draws the hexagon mark at size px: a stroked hexagon, a filled
// center circle, and six spokes — matching the GoModel logo geometry.
func render(size int, stroke color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 0}}, image.Point{}, draw.Src)

	c := float64(size) / 2
	hexR := float64(size) * 0.42
	hex := make([][2]float64, 6)
	for i := 0; i < 6; i++ {
		angle := math.Pi/2 + float64(i)*math.Pi/3
		hex[i] = [2]float64{c + hexR*math.Cos(angle), c + hexR*math.Sin(angle)}
	}

	strokeW := float64(size) * 0.07
	for i := 0; i < 6; i++ {
		a, b := hex[i], hex[(i+1)%6]
		stampLine(img, a, b, strokeW, stroke)
	}

	// Center circle.
	circleR := float64(size) * 0.13
	stampCircle(img, c, c, circleR, stroke)

	// Spokes from the circle edge toward each vertex.
	spokeW := strokeW * 0.75
	for i := 0; i < 6; i++ {
		angle := math.Pi/2 + float64(i)*math.Pi/3
		from := [2]float64{c + circleR*math.Cos(angle), c + circleR*math.Sin(angle)}
		to := hex[i]
		stampLine(img, from, to, spokeW, stroke)
	}
	return img
}

func stampLine(img *image.RGBA, from, to [2]float64, w float64, col color.RGBA) {
	dx, dy := to[0]-from[0], to[1]-from[1]
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	steps := int(length / 2)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		stampCircle(img, from[0]+dx*t, from[1]+dy*t, w/2, col)
	}
}

func stampCircle(img *image.RGBA, cx, cy, r float64, col color.RGBA) {
	minX, maxX := int(math.Floor(cx-r)), int(math.Ceil(cx+r))
	minY, maxY := int(math.Floor(cy-r)), int(math.Ceil(cy+r))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if math.Hypot(float64(x)-cx, float64(y)-cy) > r {
				continue
			}
			if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
				continue
			}
			img.SetRGBA(x, y, col)
		}
	}
}

func writePNG(path string, img *image.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func fatal(err error) {
	_, _ = os.Stderr.WriteString("genicon: " + err.Error() + "\n")
	os.Exit(1)
}
