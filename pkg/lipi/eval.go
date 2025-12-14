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
	"regexp"
	"strings"
)

const (
	strregx = `\{([a-zA-Z0-9_+\-*/]+)\}`
)

var (
	strre = regexp.MustCompile(strregx)
)

func startsWith(list []Value, s Symbol) bool {
	if len(list) > 0 {
		switch i := list[0].(type) {
		case Symbol:
			return s == i
		default:
		}
	}
	return false
}

func quasiQuoteLoop(args []Value) Value {
	var p []Value
	for i := len(args) - 1; 0 <= i; i -= 1 {
		v := args[i]
		switch v := v.(type) {
		case []Value:
			if startsWith(v, "splice-unquote") {
				p = []Value{Symbol("concat"), v[1], p}
				continue
			}
		default:
		}
		p = []Value{Symbol("cons"), quasiQuote(v), p}
	}
	return p
}

func quasiQuote(c Value) Value {
	switch c := c.(type) {
	case Symbol, Map:
		return []Value{Symbol("quote"), c}
	case []Value:
		if startsWith(c, Symbol("unquote")) {
			return c[1]
		} else {
			return quasiQuoteLoop(c)
		}
	default:
		return c
	}
}

func evalMap(args []Value, scope *Scope) ([]Value, error) {
	var result []Value
	for _, p := range args {
		expr, err := EvalCapsuleInScope(p, scope)
		if err != nil {
			return nil, err
		}
		result = append(result, expr)
	}
	return result, nil
}

func EvalCapsuleInScope(c Value, scope *Scope) (Value, error) {
	for {
		if id, ok := c.(Symbol); ok {
			return scope.Get(id)
		} else if _, ok := c.(Map); ok {
			evaluatedMap := map[string]Value{}
			for key, value := range c.(Map) {
				evaluatedValue, err := EvalCapsuleInScope(value, scope)
				if err != nil {
					return nil, err
				}
				evaluatedMap[key] = evaluatedValue
			}
			return evaluatedMap, nil
		} else if str, ok := c.(string); ok {
			var errors []string
			result := strre.ReplaceAllStringFunc(str, func(s string) string {
				matches := strre.FindStringSubmatch(s)
				if len(matches) > 1 {
					value, err := scope.Get(Symbol(matches[1]))
					if err != nil {
						errors = append(errors, fmt.Sprintf("unbound variable %v", matches[1]))
						return ""
					}
					return ToString(value)
				}
				return s
			})
			if len(errors) != 0 {
				return nil, fmt.Errorf("failed to eval string %v", strings.Join(errors, ", "))
			}
			return result, nil
		} else if _, ok := c.([]Value); !ok {
			return c, nil
		} else {
			if len(c.([]Value)) == 0 {
				return c, nil
			}

			callee := c.([]Value)[0]
			args := c.([]Value)[1:]

			callee_id := ""
			if _, ok := callee.(Symbol); ok {
				callee_id = string(callee.(Symbol))
			}

			switch callee_id {
			case "define":
				if err := CheckArgs("define", args, []checker{
					HasExactCount(2),
					EitherOf(
						OfKinds(SymbolKind, AnyKind),
						OfKinds(ListKind, AnyKind),
					),
				}); err != nil {
					return nil, err
				}

				if id, ok := args[0].(Symbol); ok {
					value, err := EvalCapsuleInScope(args[1], scope)
					if err != nil {
						return nil, err
					}
					return scope.Set(id, value), nil
				} else if parameters, ok := args[0].([]Value); ok {
					if err := CheckArgs("<define>", parameters, []checker{
						HasAtleast(1),
						AllOfKind(SymbolKind),
					}); err != nil {
						return nil, err
					}

					id := parameters[0].(Symbol)
					parameters = parameters[1:]

					return scope.Set(id, Function{
						Args:    parameters,
						Body:    args[1],
						Scope:   scope,
						IsMacro: false,
					}), nil
				} else {
					return nil, fmt.Errorf("<define> unexpected args")
				}

			case "let":
				if err := CheckArgs("let", args, []checker{
					HasExactCount(2),
					OfKinds(ListKind),
				}); err != nil {
					return nil, err
				}

				nestedScope, err := createNestedScope(scope, nil, nil)
				if err != nil {
					return nil, err
				}

				bindings := args[0].([]Value)
				for i, b := range bindings {
					pair, ok := b.([]Value)
					if !ok || len(pair) != 2 {
						return nil, fmt.Errorf("<let> binding %d must be (symbol expr)", i)
					}
					id, ok := pair[0].(Symbol)
					if !ok {
						return nil, fmt.Errorf("<let> binding %d must be (symbol expr)", i)
					}

					val, err := EvalCapsuleInScope(pair[1], scope)
					if err != nil {
						return nil, err
					}
					nestedScope.Set(id, val)
				}

				c = args[1]
				scope = nestedScope
			case "quote":
				return args[0], nil
			case "quasiquote":
				c = quasiQuote(args[0])
			case "define-macro":
				if err := CheckArgs("<define-macro>", args, []checker{
					HasExactCount(2),
					OfKinds(ListKind, AnyKind),
				}); err != nil {
					return nil, err
				}

				macroArgs := args[0].([]Value)

				if err := CheckArgs("<define-macro> ARGS", macroArgs, []checker{
					HasAtleast(1),
					AllOfKind(SymbolKind),
				}); err != nil {
					return nil, err
				}

				id := macroArgs[0].(Symbol)
				macroArgs = macroArgs[1:]

				return scope.Set(id, Function{
					Args:    macroArgs,
					Body:    args[1],
					Scope:   scope,
					IsMacro: true,
				}), nil
			case "if":
				if err := CheckArgs("if", args, []checker{
					HasAtleast(2),
				}); err != nil {
					return nil, err
				}
				cond, err := EvalCapsuleInScope(args[0], scope)
				if err != nil {
					return nil, err
				}
				if cond == nil || cond == false {
					if len(args) == 3 {
						c = args[2]
					} else {
						return nil, nil
					}
				} else {
					c = args[1]
				}
			case "do":
				if len(args) == 0 {
					return nil, nil
				}
				_, err := evalMap(args[:len(args)-1], scope)
				if err != nil {
					return nil, err
				}
				if len(args) == 1 {
					return nil, nil
				}
				c = args[len(args)-1]
			case "fun":
				if err := CheckArgs("fun", args, []checker{
					HasExactCount(2),
					OfKinds(ListKind, AnyKind),
				}); err != nil {
					return nil, err
				}

				if err := CheckArgs("<fun>", args[0].([]Value), []checker{
					AllOfKind(SymbolKind),
				}); err != nil {
					return nil, err
				}

				return Function{
					Args:    args[0],
					Body:    args[1],
					Scope:   scope,
					IsMacro: false,
				}, nil

			default:
				value, err := EvalCapsuleInScope(callee, scope)
				if err != nil {
					return nil, err
				}

				if lambda, ok := value.(Function); ok && lambda.IsMacro {
					nestedScope, err := createNestedScope(lambda.Scope, lambda.Args, args)
					if err != nil {
						return nil, err
					}
					c, err = EvalCapsuleInScope(lambda.Body, nestedScope)
					if err != nil {
						return nil, err
					}
					continue
				}

				var exprs []Value
				for _, arg := range args {
					expr, err := EvalCapsuleInScope(arg, scope)
					if err != nil {
						return nil, err
					}
					exprs = append(exprs, expr)
				}

				if lambda, ok := value.(Function); ok {
					scope, err = createNestedScope(lambda.Scope, lambda.Args, exprs)
					if err != nil {
						return nil, err
					}
					c = lambda.Body
				} else if process, ok := value.(Process); ok {
					return process(exprs)
				} else {
					return nil, fmt.Errorf("non-callable capsule: %v", value)
				}
			}
		}
	}
}

func Apply(callee Value, args ...Value) (Value, error) {
	node := []Value{callee}
	node = append(node, args...)
	return EvalCapsuleInScope(node, Global)
}

func Eval(source string) (Value, error) {
	return EvalInScope(source, Global)
}

func EvalInScope(source string, scope *Scope) (Value, error) {
	cap, err := Read(source)
	if err != nil {
		return nil, err
	}
	return EvalCapsuleInScope(cap, Global)
}
