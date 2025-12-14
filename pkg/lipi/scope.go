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
	_ "embed"
	"fmt"
)

type Scope struct {
	store  map[Symbol]Value
	parent *Scope
}

var (
	Global *Scope
)

//go:embed core.lipi
var coreLipi string

func init() {
	Global = &Scope{
		store:  map[Symbol]Value{},
		parent: nil,
	}

	registerBuiltins(Global)
	EvalInScope(coreLipi, Global)
}

func createNestedScope(parent *Scope, bindingsVal Value, exprsVal Value) (*Scope, error) {
	s := &Scope{
		store:  make(map[Symbol]Value),
		parent: parent,
	}

	if bindingsVal == nil && exprsVal == nil {
		return s, nil
	}

	bindings, ok := bindingsVal.(List)
	if !ok {
		return nil, fmt.Errorf("expected bindings to be List")
	}

	exprs, ok := exprsVal.(List)
	if !ok {
		return nil, fmt.Errorf("expected exprs to be List")
	}

	i := 0
	for i < len(bindings) {
		b, ok := bindings[i].(Symbol)
		if !ok {
			return nil, fmt.Errorf("binding %v is not a symbol", bindings[i])
		}

		if b == "." {
			if i+1 >= len(bindings) {
				return nil, fmt.Errorf("dot must be followed by a symbol")
			}

			restSym, ok := bindings[i+1].(Symbol)
			if !ok {
				return nil, fmt.Errorf("variadic name must be a symbol")
			}

			s.store[restSym] = exprs[i:]
			return s, nil
		}

		if i >= len(exprs) {
			return nil, fmt.Errorf("not enough arguments for bindings")
		}

		s.store[b] = exprs[i]
		i++
	}

	if len(exprs) > len(bindings) {
		return nil, fmt.Errorf("too many arguments")
	}

	return s, nil
}

func (s *Scope) Lookup(id Symbol) *Scope {
	if _, ok := s.store[id]; ok {
		return s
	} else if s.parent != nil {
		return s.parent.Lookup(id)
	}
	return nil
}

func (s *Scope) Set(id Symbol, c Value) Value {
	s.store[id] = c
	return c
}

func (s *Scope) Get(id Symbol) (Value, error) {
	scope := s.Lookup(id)
	if scope == nil {
		return nil, fmt.Errorf("unbounded value %v", id)
	}
	return scope.store[id], nil
}
