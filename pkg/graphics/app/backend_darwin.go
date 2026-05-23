//go:build darwin

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

package app

import (
	"runtime"

	"avyos.dev/pkg/graphics/backend"
	"avyos.dev/pkg/graphics/backend/macos"
)

func init() { runtime.LockOSThread() }

// DefaultBackend returns the Cocoa/AppKit backend for macOS.
func DefaultBackend() backend.Backend { return macos.New() }
