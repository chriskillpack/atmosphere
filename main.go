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

func (c Color) Multiply(f float64) Color {
	// TODO - why do we have to set A to 255 and not pass it through?
	// Something to do with premultiplied alpha?
	return Color{c.R * f, c.G * f, c.B * f, 255}
}

// Convert the color to color.RGBA and does [0,255] clamping
func (c Color) Pack() color.RGBA {
	uR := uint8(clamp(c.R*255, 0, 255))
	uG := uint8(clamp(c.G*255, 0, 255))
	uB := uint8(clamp(c.B*255, 0, 255))
	uA := uint8(clamp(c.A*255, 0, 255))

	return color.RGBA{uR, uG, uB, uA}
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

// Crude numerical integrator using trapezoidal quadrature, currently unused
// For now we assume that atmospheric density is constant through atmosphere which
// means we can avoid numerical integration. At some point should use height relative
// density, see Nishita "Display of The Earth Taking into Account Atmospheric Scattering"
// section 4.2, http://nishitalab.org/user/nis/cdrom/sig93_nis.pdf
func numIntegrate(fn func(float64) float64, a, b float64, n int) float64 {
	dx := (b - a) / float64(n)

	var area float64
	x := a
	prevfn := fn(x)
	for i := 0; i < n; i++ {
		newx := x + dx
		newfn := fn(newx)
		area = area + (dx * (prevfn + newfn) / 2)

		x = newx
		prevfn = newfn
	}

	return area
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

	// fmt.Println(numIntegrate(curve, 1.0, 5.0, 4))
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))

	so := Sphere{Vector3{0, 0, 50}, EarthRadius + EarthAtmosphereHeight, Identity()}
	si := Sphere{Vector3{0, 0, 50}, EarthRadius, Rotate(Vector3{0, 1, 0}, -3.1)}

	for y := 0; y < 480; y++ {
		for x := 0; x < 640; x++ {
			var dir Vector3
			dir.X = (float64(x-ImageWidth/2) / (ImageWidth / 2)) * (float64(ImageWidth) / 480)
			dir.Y = float64(y-ImageHeight/2) / (ImageHeight / 2)
			dir.Z = 5

			r := Ray{Vector3{0, 0, -40 * 1000 * 1000}, dir.Normalize()}
			// Does it hit the planet out atmosphere?
			ho := so.Intersect(r)
			if ho == NoHit {
				img.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				// Does it intersect inner sphere?

				// Advance along ray very slightly to avoid intersecting
				// planet atmosphere again
				t1 := NextFloatUp(ho.T)
				// Compute start point for the ray
				ri := Ray{r.Direction.Multiply(t1).Add(r.Origin), r.Direction}

				// Does it hit inner sphere?
				hi := si.Intersect(ri)
				var c color.RGBA
				if hi == NoHit {
					// No, but it will may hit ho again so let's see how far through
					// the outer sphere the ray travels until it exits
					ho2 := so.Intersect(ri)
					if ho2 == NoHit {
						// Initial contact just grazed the outer atmosphere
						c = color.RGBA{255, 255, 255, 255}
					} else {
						// Find the linear distance travelled across the sphere
						linDist := math.Min(ho2.T/(EarthAtmosphereHeight*60)*255, 255)
						ld := uint8(linDist)
						c = color.RGBA{ld, ld, ld, 255}
					}
				} else {
					// Compute contact point in world space
					cp := ri.Direction.Multiply(hi.T).Add(ri.Origin)
					uv := si.UV(cp)

					// Shade the point with directional sunlight
					n := si.Normal(cp)
					n = si.Transform.MulDirection(n)

					l := math.Max(0, -n.Dot(SunlightDir)) * SunlightIntensity

					// Apply sunlight amount to earth albedo texture
					tc := sampleTexture(tex, uv.X, uv.Y)
					tc = tc.Multiply(l)
					c = tc.Pack()
				}

				img.SetRGBA(x, y, c)
			}
		}
	}
	of, err := os.Create("./image.png")
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
