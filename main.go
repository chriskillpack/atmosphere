// Atmospheric scattering

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"unsafe"
)

const (
	ImageWidth  = 640
	ImageHeight = 480

	EarthRadius           = 6371000 // meters
	EarthAtmosphereHeight = 100000  // meters
)

type Ray struct {
	Origin    Vector3
	Direction Vector3
}

type Hit struct {
	Shape Shape
	T     float64
}

var (
	NoHit             = Hit{nil, 1e9}
	SunlightDir       = Vector3{3, 5, 1}.Normalize()
	SunlightIntensity = 3.0

	// Rayleight extinction coefficients computed for R, G and B wavelengths.
	// We use the wavelengths from Hoffman and Preetham of [650, 570, 475]nm and matched
	// our extinction coefficients to theirs.
	RayleighExtinction   = Color{6.95265e-06, 1.17572e-05, 2.43797e-05, 0}
	RayleighDensityScale = 0.25

	// Mie extinction coefficients for R, G and B wavelengths.
	// These values were taken from Bruneton
	MieExtinction   = Color{2.3e-06, 2.3e-06, 2.3e-06, 0}
	MieDensityScale = 0.1
)

type Shape interface {
	// Test if the world space ray hit the object
	Intersect(Ray) Hit
	// Given a position in world space compute and return UV coordinates in X & Y components
	UV(Vector3) Vector3
	// Given a position in world space return the normal in object space
	Normal(Vector3) Vector3
}

type Sphere struct {
	Origin    Vector3
	Radius    float64
	Transform Matrix
}

type Color struct {
	R, G, B, A float64
}

func (a Color) AddRGB(b Color) Color {
	return Color{a.R + b.R, a.G + b.G, a.B + b.B, a.A}
}

func (c Color) MultiplyRGB(f float64) Color {
	return Color{c.R * f, c.G * f, c.B * f, c.A}
}

// Convert the color to color.RGBA and does [0,255] clamping
func (c Color) Pack() color.NRGBA {
	uR := uint8(clamp(c.R*255, 0, 255))
	uG := uint8(clamp(c.G*255, 0, 255))
	uB := uint8(clamp(c.B*255, 0, 255))
	uA := uint8(clamp(c.A*255, 0, 255))

	return color.NRGBA{uR, uG, uB, uA}
}

func NewColorFromRGBA(r, g, b, a uint32) Color {
	return Color{float64(r) / 65536, float64(g) / 65536, float64(b) / 65536, float64(a) / 65536}
}

var _ Shape = &Sphere{}

// From PBRT 3rd ed pg 212
func NextFloatUp(v float64) float64 {
	if math.IsInf(v, 1) && v > 0 {
		return v
	}
	if v == -0.0 {
		return 0.0
	}

	ui := *(*uint64)(unsafe.Pointer(&v))
	if v >= 0 {
		ui++
	} else {
		ui--
	}

	return *(*float64)(unsafe.Pointer(&ui))
}

// From https://github.com/fogleman/pt/blob/69e74a07b0af72f1601c64120a866d9a5f432e2f/pt/sphere.go#L26-L43
func (s Sphere) Intersect(r Ray) Hit {
	// Ray is in world space, transform the ray into local space
	or := s.Transform.Inverse().MulRay(r)

	to := or.Origin.Sub(s.Origin)
	b := to.Dot(or.Direction)
	c := to.Dot(to) - s.Radius*s.Radius
	d := b*b - c
	if d > 0 {
		d = math.Sqrt(d)
		t1 := -b - d
		if t1 > 1e-5 {
			return Hit{s, t1}
		}
		t2 := -b + d
		if t2 > 1e-5 {
			return Hit{s, t2}
		}
	}

	return NoHit
}

// p is in shape coordinate space
// returned vector only sets x & y components for u & v coords
// From https://github.com/fogleman/pt/blob/69e74a07b0af72f1601c64120a866d9a5f432e2f/pt/sphere.go#L45-L52
func (s Sphere) UV(wp Vector3) Vector3 {
	p := s.Transform.Inverse().MulPosition(wp)
	p = p.Sub(s.Origin)
	u := math.Atan2(p.Z, p.X)
	v := math.Atan2(p.Y, Vector3{p.X, 0, p.Z}.Length())
	u = 1 - (u+math.Pi)/(2*math.Pi)
	v = (v + math.Pi/2) / math.Pi
	return Vector3{u, v, 0}
}

func (s Sphere) Normal(wp Vector3) Vector3 {
	p := s.Transform.Inverse().MulPosition(wp)
	p = p.Sub(s.Origin)
	return p.Normalize()
}

// Numerical integrator using the trapezoidal rule
// Integrates fn(x) over the domain [a,b] in n steps
func numIntegrate(fn func(float64) float64, a, b float64, n int) float64 {
	dx := (b - a) / float64(n-1)

	var area float64
	x := a
	prevfn := fn(x)
	for i := 1; i < n; i++ {
		newx := x + dx
		newfn := fn(newx)
		area += prevfn + newfn

		x = newx
		prevfn = newfn
	}

	return (area * dx) * 0.5
}

func clamp(x, min, max float64) float64 {
	return math.Max(math.Min(x, max), min)
}

// Nearest neighbor
func sampleTexture(img image.Image, u, v float64) Color {
	bounds := img.Bounds()
	x := int(clamp(u, 0, 1) * float64(bounds.Max.X))
	y := int(clamp(v, 0, 1) * float64(bounds.Max.Y))
	r, g, b, a := img.At(x, y).RGBA()
	return NewColorFromRGBA(r, g, b, a)
}

func main() {
	f, err := os.Open("earth.png")
	if err != nil {
		fmt.Printf("err reading 'earth.png': %v\n", err)
		os.Exit(1)
	}
	tex, err := png.Decode(f)
	if err != nil {
		fmt.Printf("err reading 'earth.png': %v\n", err)
		f.Close()
		os.Exit(1)
	}
	f.Close()

	img := image.NewRGBA(image.Rect(0, 0, ImageWidth, ImageHeight))

	so := Sphere{Vector3{0, 0, 50}, EarthRadius + EarthAtmosphereHeight, Identity()}
	si := Sphere{Vector3{0, 0, 50}, EarthRadius, Rotate(Vector3{0, 1, 0}, -3.1)}

	for y := 0; y < ImageHeight; y++ {
		for x := 0; x < ImageWidth; x++ {
			var dir Vector3
			dir.X = (float64(x-ImageWidth/2) / (ImageWidth / 2)) * (float64(ImageWidth) / ImageHeight)
			dir.Y = float64(y-ImageHeight/2) / (ImageHeight / 2)
			dir.Z = 5

			c := Color{0, 0, 0, 1}
			r := Ray{Vector3{0, 0, -40 * 1000 * 1000}, dir.Normalize()}

			// Does it hit the planet out atmosphere?
			ho := so.Intersect(r)
			if ho != NoHit {
				// Advance along ray very slightly to avoid intersecting
				// planet atmosphere again
				t1 := NextFloatUp(ho.T)
				// Compute start point for the ray
				ri := Ray{r.Direction.Multiply(t1).Add(r.Origin), r.Direction}

				var olE float64

				// Does it hit the planet?
				hi := si.Intersect(ri)
				if hi != NoHit {
					// Optical length calculation ends at the planet
					olE = hi.T

					// Compute contact point in world space
					cp := ri.Direction.Multiply(hi.T).Add(ri.Origin)
					uv := si.UV(cp)

					// Shade the point with directional sunlight
					n := si.Normal(cp)
					n = si.Transform.MulDirection(n)

					l := math.Max(0, -n.Dot(SunlightDir)) * SunlightIntensity

					// Apply sunlight amount to earth albedo texture
					c = sampleTexture(tex, uv.X, uv.Y)
					c = c.MultiplyRGB(l)
				} else {
					// Did not hit planet, compute where it hits outer atmosphere
					ho2 := so.Intersect(ri)
					if ho2 != NoHit {
						olE = ho2.T
					}
					// If it did not hit then the first ray grazed the atmosphere and we take the end
					// point to be the same as the start point, 0
				}

				// Compute optical length along the ray
				// Using https://developer.nvidia.com/gpugems/GPUGems2/gpugems2_chapter16.html as a guide
				optLengthFn := func(t float64) float64 {
					p := ri.Direction.Multiply(t).Add(ri.Origin)
					h := (p.Sub(si.Origin).Length() - si.Radius) / (so.Radius - si.Radius)
					return math.Exp(-h / RayleighDensityScale)
				}
				optLength := numIntegrate(optLengthFn, 0, olE, 5)

				// Perform extinction due to absorbtion and outscattering
				c.R = c.R * math.Exp(-RayleighExtinction.R*optLength)
				c.G = c.G * math.Exp(-RayleighExtinction.G*optLength)
				c.B = c.B * math.Exp(-RayleighExtinction.B*optLength)
			}
			img.Set(x, y, c.Pack())
		}
	}
	of, err := os.Create("./out.png")
	if err != nil {
		fmt.Printf("Could not create output file: %v", err)
		return
	}
	defer of.Close()
	err = png.Encode(of, img)
	if err != nil {
		fmt.Printf("Encode PNG failed %v", err)
		return
	}
}
