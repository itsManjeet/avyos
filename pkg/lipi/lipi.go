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
	"reflect"
	"strings"
)

type Value interface {
}

type Symbol string

func NewKeyword(s string) Symbol {
	return Symbol("\u029e" + strings.ToUpper(s))
}

func IsKeyword(c Value) bool {
	s, ok := c.(Symbol)
	return ok && strings.HasPrefix(string(s), "\u029e")
}

type Process = func([]Value) (Value, error)

type Function struct {
	Args    Value
	Body    Value
	Scope   *Scope
	IsMacro bool
}

type List = []Value

func listToMap(l List) (Map, error) {
	if len(l)%2 == 1 {
		return nil, fmt.Errorf("expect even numer of values")
	}
	m := Map{}
	for i := 0; i < len(l); i += 2 {
		s, ok := l[i].(string)
		if !ok {
			return nil, fmt.Errorf("expect map key as string")
		}
		m[s] = l[i+1]
	}

	return m, nil
}

type Map = map[string]Value

func IsEqual(a, b Value) bool {
	at := reflect.TypeOf(a)
	bt := reflect.TypeOf(b)

	isList := func(c Value) bool {
		_, ok := c.(List)
		return ok
	}

	if !((at == bt) || (isList(a) && isList(b))) {
		return false
	}

	switch a.(type) {
	case Symbol:
		return a.(Symbol) == b.(Symbol)
	case List:
		a := a.(List)
		b := b.(List)
		if len(a) != len(b) {
			return false
		}

		for i := 0; i < len(a); i++ {
			if !IsEqual(a[i], b[i]) {
				return false
			}
		}
		return true
	case Map:
		a := a.(Map)
		b := b.(Map)
		if len(a) != len(b) {
			return false
		}

		for key, value := range a {
			if !IsEqual(value, b[key]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}
