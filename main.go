// Atmospheric scattering

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
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
	SunlightDir       = Vector3{3, -5, 1}.Normalize()
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

var debugIntersect bool

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

func nextFloatUp(v float64) float64 {
	return math.Nextafter(v, math.Inf(1))
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
	u = (u + math.Pi) / (2 * math.Pi)
	v = (math.Pi - (v + math.Pi/2)) / math.Pi
	return Vector3{u, v, 0}
}

func (s Sphere) Normal(wp Vector3) Vector3 {
	p := s.Transform.Inverse().MulPosition(wp)
	p = p.Sub(s.Origin)
	return p.Normalize()
}

// Numerical integrator using the trapezoidal rule
// Integrates scalar function fn(x) over the domain [a,b] in n steps
func numIntegrate(fn func(_, _ float64) float64, a, b float64, n int) float64 {
	dx := (b - a) / float64(n-1)

	var area float64
	x := a
	prevfn := fn(x, dx)
	for i := 1; i < n; i++ {
		newx := x + dx
		newfn := fn(newx, dx)
		area += prevfn + newfn

		x = newx
		prevfn = newfn
	}

	return (area * dx) * 0.5
}

// Same as numIntegrate but integrates a vector function fn(x)
func numIntegrateV(fn func(_, _ float64) Vector3, a, b float64, n int) Vector3 {
	dx := (b - a) / float64(n-1)

	var area Vector3
	x := a
	prevfn := fn(x, dx)
	for i := 1; i < n; i++ {
		newx := x + dx
		newfn := fn(newx, dx)
		area = area.Add(prevfn.Add(newfn))

		x = newx
		prevfn = newfn
	}

	return area.Multiply(dx * 0.5)
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

	// World space -> Camera space
	// Increase World X -> Move right in the camera
	// Increase World Y -> Move up in the camera
	// Increase World Z -> Move away from the camera (into screen)
	so := Sphere{Vector3{0, 0, 0}, EarthRadius + EarthAtmosphereHeight, Identity()}
	si := Sphere{Vector3{0, 0, 0}, EarthRadius, Rotate(Vector3{0, 1, 0}, -0.5)}

	for y := 0; y < ImageHeight; y++ {
		for x := 0; x < ImageWidth; x++ {
			var dir Vector3
			dir.X = (float64(x-ImageWidth/2) / (ImageWidth / 2)) * (float64(ImageWidth) / ImageHeight)
			dir.Y = float64(ImageHeight/2-y) / (ImageHeight / 2)
			dir.Z = 5

			c := Color{0, 0, 0, 1}
			r := Ray{Vector3{0, 0, -40 * 1000 * 1000}, dir.Normalize()}

			// Does it hit the planet outer atmosphere?
			debugIntersect = x == 320 && (y == 400 || y == 80 || y == 240)
			debugIntersect = false
			if debugIntersect {
				fmt.Printf("y %v\n", y)
			}

			// Ray definitions
			// r - the starting ray from the camera into the scene
			// ri - from the hit point on outer atmosphere this ray is in the same direction
			//   as r. used to find if the view ray hits the planet or exits the atmosphere
			// rs - ray from a point in the atmosphere back towards the sun
			// rc - ray from a point back towards the camera

			// Does it hit the planet outer atmosphere?
			ho := so.Intersect(r)
			if ho != NoHit {
				// Advance along ray very slightly to avoid intersecting
				// planet atmosphere again
				t1 := nextFloatUp(ho.T)
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

					// Some temporary lighting from the sun (this needs to be tweaked)
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
				optLengthFn := func(ray Ray) func(t, dx float64) float64 {
					return func(t, _ float64) float64 {
						p := ray.Direction.Multiply(t).Add(ray.Origin)
						h := (p.Sub(si.Origin).Length() - si.Radius) / (so.Radius - si.Radius)
						return math.Exp(-h / RayleighDensityScale)
					}
				}

				// First attempt at computing in-scattering term
				inScatterFn := func(t, dx float64) Vector3 {
					p := ri.Direction.Multiply(t).Add(ri.Origin)

					// First off, is this point in the shadow of the planet?
					rshd := Ray{p, Vector3{-SunlightDir.X, -SunlightDir.Y, -SunlightDir.Z}}
					rshdHit := si.Intersect(rshd)
					if rshdHit != NoHit {
						// Yes, no contributions (for now)
						if debugIntersect {
							fmt.Printf("In shadow of planet\n")
						}
					} else {
						// Fire a ray from p towards the sun, see how far to the outer atmosphere
						rs := Ray{p, Vector3{-SunlightDir.X, -SunlightDir.Y, -SunlightDir.Z}}
						rsHit := so.Intersect(rs)
						if rsHit != NoHit {
							// Compute optical length along the sunlight ray from p to the edge of the atmosphere
							sunOptLength := numIntegrate(optLengthFn(rs), 0, rsHit.T, 5)

							// Determine how much sunlight reaches the point. It gets attenuated as it
							// passes through the atmosphere. To keep things simple We ignore in scattering
							// events along this path.
							fudge := 1e-5 // TODO - Can I eliminate this?
							sunColor := Vector3{
								SunlightIntensity * math.Exp(-RayleighExtinction.R*sunOptLength) * fudge,
								SunlightIntensity * math.Exp(-RayleighExtinction.G*sunOptLength) * fudge,
								SunlightIntensity * math.Exp(-RayleighExtinction.B*sunOptLength) * fudge,
							}

							// Compute contribution of sunlight to path
							cosT := r.Direction.Dot(SunlightDir)
							scatPhase := (3 / (16.0 * math.Pi)) * (cosT*cosT + 1)
							contrib := sunColor.Multiply(scatPhase)

							// It undergoes extinction on the path segment
							// My intuition is to use the step size between integration samples as the distance
							// travelled because we are accumulating in-scattering events along the entire path.
							// TODO - verify
							return Vector3{
								contrib.X * math.Exp(-RayleighExtinction.R*dx),
								contrib.Y * math.Exp(-RayleighExtinction.G*dx),
								contrib.Z * math.Exp(-RayleighExtinction.B*dx),
							}
						} else {
							// Calling out an exceptional case - this should never be reached
							// TODO: we are getting here, this needs to be debugged
							// fmt.Printf("What am I doing here?\n")
						}
					}
					return Vector3{}
				}
				inScatter := numIntegrateV(inScatterFn, 0, olE, 50)
				inScatterCol := Color{inScatter.X, inScatter.Y, inScatter.Z, 1}

				// Final color = planet color * Fex + Fin
				// TODO - include Fex term
				c = c.AddRGB(inScatterCol)
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
