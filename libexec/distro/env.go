/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
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

import (
	"strings"
)

func setEnvKV(env []string, kv string) []string {
	kv = strings.TrimSpace(kv)
	if kv == "" {
		return env
	}

	idx := strings.IndexByte(kv, '=')
	if idx <= 0 {
		return append(env, kv)
	}

	key := kv[:idx]
	prefix := key + "="
	for i := range env {
		if strings.HasPrefix(env[i], prefix) {
			env[i] = kv
			return env
		}
	}
	return append(env, kv)
}

func setEnv(env []string, key, value string) []string {
	return setEnvKV(env, key+"="+value)
}

func setEnvMany(env []string, entries []string) []string {
	for _, entry := range entries {
		env = setEnvKV(env, entry)
	}
	return env
}
