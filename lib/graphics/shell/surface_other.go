//go:build !linux

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

package shell

import "image"

// surface is a stub on non-Linux platforms where shared-memory buffers are not
// available. All operations are no-ops and image() always returns nil.
type surface struct{}

func newSurface() *surface { return &surface{} }

func (s *surface) setBuffer(path string, width, height, scaleMilli int) error { return nil }
func (s *surface) image() image.Image                                         { return nil }
func (s *surface) lockImage() image.Image                                     { return nil }
func (s *surface) unlockImage()                                               {}
func (s *surface) Close()                                                     {}
