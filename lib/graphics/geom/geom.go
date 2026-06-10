// Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

// Package geom provides fundamental 2D geometry primitives.
package geom

import "math"

// Point is a 2D point or vector.
type Point struct{ X, Y float64 }

// Pt constructs a Point from x and y.
func Pt(x, y float64) Point { return Point{x, y} }

// Add returns the vector sum p + q.
func (p Point) Add(q Point) Point { return Point{p.X + q.X, p.Y + q.Y} }

// Sub returns the vector difference p − q.
func (p Point) Sub(q Point) Point { return Point{p.X - q.X, p.Y - q.Y} }

// Scale returns p scaled by scalar s.
func (p Point) Scale(s float64) Point { return Point{p.X * s, p.Y * s} }

// Neg returns the negation of p.
func (p Point) Neg() Point { return Point{-p.X, -p.Y} }

// Len returns the Euclidean length of p.
func (p Point) Len() float64 { return math.Sqrt(p.X*p.X + p.Y*p.Y) }

// Dot returns the dot product of p and q.
func (p Point) Dot(q Point) float64 { return p.X*q.X + p.Y*q.Y }

// Lerp linearly interpolates from p to q by t ∈ [0, 1].
func (p Point) Lerp(q Point, t float64) Point {
	return Point{p.X + (q.X-p.X)*t, p.Y + (q.Y-p.Y)*t}
}

// Size represents width and height dimensions.
type Size struct{ Width, Height float64 }

// Sz constructs a Size from width and height.
func Sz(w, h float64) Size { return Size{w, h} }

// Area returns Width × Height.
func (s Size) Area() float64 { return s.Width * s.Height }

// Rect is an axis-aligned rectangle defined by its minimum and maximum corners.
type Rect struct{ Min, Max Point }

// NewRect constructs a Rect from a top-left origin (x, y) and dimensions (w, h).
func NewRect(x, y, w, h float64) Rect { return Rect{Min: Pt(x, y), Max: Pt(x+w, y+h)} }

// FromPoints constructs a Rect directly from two corner points.
func FromPoints(min, max Point) Rect { return Rect{Min: min, Max: max} }

// Width returns the horizontal span of the rectangle.
func (r Rect) Width() float64 { return r.Max.X - r.Min.X }

// Height returns the vertical span of the rectangle.
func (r Rect) Height() float64 { return r.Max.Y - r.Min.Y }

// Size returns the width and height as a Size value.
func (r Rect) Size() Size { return Size{r.Width(), r.Height()} }

// Center returns the midpoint of the rectangle.
func (r Rect) Center() Point { return Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2) }

// Empty reports whether the rectangle has zero or negative area.
func (r Rect) Empty() bool { return r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y }

// Contains reports whether p lies inside the rectangle (Min inclusive, Max exclusive).
func (r Rect) Contains(p Point) bool {
	return p.X >= r.Min.X && p.X < r.Max.X && p.Y >= r.Min.Y && p.Y < r.Max.Y
}

// Intersect returns the largest rectangle contained in both r and s.
func (r Rect) Intersect(s Rect) Rect {
	return Rect{
		Min: Pt(math.Max(r.Min.X, s.Min.X), math.Max(r.Min.Y, s.Min.Y)),
		Max: Pt(math.Min(r.Max.X, s.Max.X), math.Min(r.Max.Y, s.Max.Y)),
	}
}

// Union returns the smallest rectangle that contains both r and s.
func (r Rect) Union(s Rect) Rect {
	return Rect{
		Min: Pt(math.Min(r.Min.X, s.Min.X), math.Min(r.Min.Y, s.Min.Y)),
		Max: Pt(math.Max(r.Max.X, s.Max.X), math.Max(r.Max.Y, s.Max.Y)),
	}
}

// Inset shrinks the rectangle by dx on the left and right, and dy on the top and bottom.
// Negative values expand the rectangle.
func (r Rect) Inset(dx, dy float64) Rect {
	return Rect{Min: Pt(r.Min.X+dx, r.Min.Y+dy), Max: Pt(r.Max.X-dx, r.Max.Y-dy)}
}

// Translate shifts the rectangle by (dx, dy).
func (r Rect) Translate(dx, dy float64) Rect {
	return Rect{Min: Pt(r.Min.X+dx, r.Min.Y+dy), Max: Pt(r.Max.X+dx, r.Max.Y+dy)}
}

// Insets represent padding on four sides of a rectangle.
type Insets struct{ Top, Right, Bottom, Left float64 }

// UniformInsets creates an Insets with the same value on all four sides.
func UniformInsets(v float64) Insets { return Insets{v, v, v, v} }

// AxisInsets creates an Insets with h padding on left/right and v on top/bottom.
func AxisInsets(h, v float64) Insets { return Insets{v, h, v, h} }

// Apply shrinks rect r by these insets, returning the inner rectangle.
func (i Insets) Apply(r Rect) Rect {
	return Rect{
		Min: Pt(r.Min.X+i.Left, r.Min.Y+i.Top),
		Max: Pt(r.Max.X-i.Right, r.Max.Y-i.Bottom),
	}
}

// Matrix is a 3x3 affine transform in column-major form [a,b,c,d,tx,ty]:
//
//	| a  c  tx |
//	| b  d  ty |
//	| 0  0   1 |
type Matrix [6]float64

// Identity returns the identity transform (no-op).
func Identity() Matrix { return Matrix{1, 0, 0, 1, 0, 0} }

// IsIdentity reports whether m is (approximately) the identity matrix.
func (m Matrix) IsIdentity() bool {
	const eps = 1e-9

	return math.Abs(m[0]-1)+
		math.Abs(m[1])+
		math.Abs(m[2])+
		math.Abs(m[3]-1)+
		math.Abs(m[4])+
		math.Abs(m[5]) < eps
}

// Mul returns the matrix product m × n (apply n first, then m).
func (m Matrix) Mul(n Matrix) Matrix {
	return Matrix{
		m[0]*n[0] + m[2]*n[1],
		m[1]*n[0] + m[3]*n[1],
		m[0]*n[2] + m[2]*n[3],
		m[1]*n[2] + m[3]*n[3],
		m[0]*n[4] + m[2]*n[5] + m[4],
		m[1]*n[4] + m[3]*n[5] + m[5],
	}
}

// Transform applies the affine transform m to point p.
func (m Matrix) Transform(p Point) Point {
	return Pt(m[0]*p.X+m[2]*p.Y+m[4], m[1]*p.X+m[3]*p.Y+m[5])
}

// Translate returns a matrix that shifts by (dx, dy).
func Translate(dx, dy float64) Matrix { return Matrix{1, 0, 0, 1, dx, dy} }

// Scale returns a matrix that scales by (sx, sy) from the origin.
func Scale(sx, sy float64) Matrix { return Matrix{sx, 0, 0, sy, 0, 0} }

// Rotate returns a matrix that rotates by angle radians counter-clockwise.
func Rotate(angle float64) Matrix {
	c, s := math.Cos(angle), math.Sin(angle)
	return Matrix{c, s, -s, c, 0, 0}
}
