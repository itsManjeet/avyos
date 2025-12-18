/*
 * Copyright (c) 2025 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import "slices"

func shutdown() error {
	su.isShuttingDown = true

	revStages := slices.Clone(stages)
	slices.Reverse(revStages)

	su.foreach(func(s *Service) {
		if s.Stage == "service" {
			s.Stop(su.journal)
		}
	})

	for _, st := range revStages {
		su.foreach(func(s *Service) {
			if s.Stage == st {
				s.Stop(su.journal)
			}
		})
	}

	return nil
}
