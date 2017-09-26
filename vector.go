// Inspired by Physically Based Rendering, 3rd edition

package main

import "math"

type Vector3 struct {
	X, Y, Z float64
}

func (a Vector3) Add(b Vector3) Vector3 {
	return Vector3{a.X + b.X, a.Y + b.Y, a.Z + b.Z}
}

func (a Vector3) Sub(b Vector3) Vector3 {
	return Vector3{a.X - b.X, a.Y - b.Y, a.Z - b.Z}
}

func (v Vector3) Multiply(f float64) Vector3 {
	return Vector3{v.X * f, v.Y * f, v.Z * f}
}

func (v Vector3) Divide(f float64) Vector3 {
	inv := 1 / f
	return v.Multiply(inv)
}

func (v Vector3) LengthSquared() float64 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z
}

func (v Vector3) Length() float64 {
	return math.Sqrt(v.LengthSquared())
}

func (v Vector3) Normalize() Vector3 {
	return v.Divide(v.Length())
}

func (v Vector3) Dot(v2 Vector3) float64 {
	return v.X*v2.X + v.Y*v2.Y + v.Z*v2.Z
}

func (v Vector3) Cross(v2 Vector3) Vector3 {
	return Vector3{
		(v.Y * v2.Z) - (v.Z * v2.Y),
		(v.Z * v2.X) - (v.X * v2.Z),
		(v.X * v2.Y) - (v.Y * v2.X),
	}
}
