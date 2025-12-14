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

package lipi

import (
	"fmt"
	"strings"
)

func ToString(c Value) string {
	switch c := c.(type) {
	case nil:
		return "#N"
	case bool:
		if c {
			return "#T"
		}
		return "#F"
	case List:
		var sb strings.Builder
		sb.WriteRune('(')
		sep := ""
		for _, v := range c {
			sb.WriteString(sep + ToString(v))
			sep = " "
		}
		sb.WriteRune(')')
		return sb.String()
	case Map:
		var sb strings.Builder
		sb.WriteRune('{')
		sep := ""
		for key, value := range c {
			sb.WriteString(sep + key + ": " + ToString(value))
			sep = " "
		}
		sb.WriteRune('}')
		return sb.String()
	case Process:
		return fmt.Sprintf("#PROCESS:%v", c)
	case Function:
		return "#LAMBDA"
	default:
		return fmt.Sprint(c)
	}
}
