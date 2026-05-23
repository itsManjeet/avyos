//go:build !darwin && !linux

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
	"avyos.dev/pkg/graphics/backend"
	"avyos.dev/pkg/graphics/backend/noop"
)

// DefaultBackend returns a headless no-op backend for unsupported platforms.
func DefaultBackend() backend.Backend { return noop.New() }
