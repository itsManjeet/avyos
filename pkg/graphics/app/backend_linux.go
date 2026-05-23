//go:build linux

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
//

package app

import (
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/graphics/backend"
	desktopbackend "avyos.dev/pkg/graphics/backend/desktop"
	"avyos.dev/pkg/graphics/backend/drmkms"
)

// DefaultBackend selects the best available display backend in this order:
//  1. avyos desktop session  (dev.avyos.desktop socket reachable)
//  2. DRM/KMS direct display (fallback)
func DefaultBackend() backend.Backend {
	if Options.Layer == nil && desktopbackend.Available("") {
		return desktopbackend.New("")
	}
	return drmkms.New(fs.Resolve("device:dri/card0"))
}
