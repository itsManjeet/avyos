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

package widget

import "time"

const defaultAnimationDuration = 200 * time.Millisecond

// AnimationCurve maps a normalized progress in [0,1] to another value in [0,1].
type AnimationCurve func(t float64) float64

// Linear returns uniform progress.
func Linear(t float64) float64 { return clamp01(t) }

// EaseIn starts slowly and accelerates.
func EaseIn(t float64) float64 { t = clamp01(t); return t * t }

// EaseOut starts quickly and decelerates.
func EaseOut(t float64) float64 { t = clamp01(t); inv := 1 - t; return 1 - inv*inv }

// EaseInOut eases both the start and end.
func EaseInOut(t float64) float64 { t = clamp01(t); return t * t * (3 - 2*t) }

// Animated interpolates a float64 Value toward a target over Duration.
// Builder is called each frame with the current interpolated value.
//
// On first render the value snaps immediately. Subsequent changes animate
// from the last rendered value to the new target.
//
//	widget.Animated{
//	    Value:    boolToFloat(selected),
//	    Duration: 150 * time.Millisecond,
//	    Curve:    widget.EaseOut,
//	    Builder:  func(v float64) widget.Widget { ... },
//	}
type Animated struct {
	Value    float64
	Duration time.Duration
	Curve    AnimationCurve
	Builder  func(value float64) Widget
}

type animatedState struct {
	start    time.Time
	duration time.Duration
	curve    AnimationCurve
	from     float64
	to       float64
	current  float64
}

func (a Animated) Build(ctx BuildContext) Widget {
	if a.Builder == nil {
		return nil
	}
	if ctx.frame == nil {
		return a.Builder(a.Value)
	}

	st, ok := ctx.frame.animations[ctx.path]
	if !ok {
		st = &animatedState{
			duration: resolvedDuration(a.Duration),
			curve:    resolvedCurve(a.Curve),
			from:     a.Value,
			to:       a.Value,
			current:  a.Value,
		}
		ctx.frame.animations[ctx.path] = st
		return a.Builder(a.Value)
	}

	now := ctx.frame.currentTime()
	current := st.valueAt(now)
	st.current = current

	if st.to != a.Value {
		st.from = current
		st.to = a.Value
		st.start = now
		st.duration = resolvedDuration(a.Duration)
		st.curve = resolvedCurve(a.Curve)
	}

	value := st.valueAt(now)
	st.current = value

	if st.isAnimating(now) {
		if ctx.frame.animatingNext != nil {
			ctx.frame.animatingNext[ctx.path] = struct{}{}
		}
	} else {
		st.current = st.to
		value = st.to
	}

	return a.Builder(value)
}

func (st *animatedState) valueAt(now time.Time) float64 {
	if st == nil || st.duration <= 0 || st.start.IsZero() || st.from == st.to {
		return st.to
	}
	elapsed := now.Sub(st.start)
	if elapsed <= 0 {
		return st.from
	}
	if elapsed >= st.duration {
		return st.to
	}
	curve := resolvedCurve(st.curve)
	progress := clamp01(float64(elapsed) / float64(st.duration))
	return st.from + (st.to-st.from)*curve(progress)
}

func (st *animatedState) isAnimating(now time.Time) bool {
	if st == nil || st.from == st.to || st.duration <= 0 || st.start.IsZero() {
		return false
	}
	return now.Sub(st.start) < st.duration
}

func resolvedDuration(d time.Duration) time.Duration {
	if d <= 0 {
		return defaultAnimationDuration
	}
	return d
}

func resolvedCurve(c AnimationCurve) AnimationCurve {
	if c == nil {
		return EaseInOut
	}
	return c
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
