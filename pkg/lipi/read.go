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
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

type reader struct {
	tokens []string
	pos    int
}

func (r *reader) next() *string {
	if r.pos >= len(r.tokens) {
		return nil
	}
	token := r.tokens[r.pos]
	r.pos = r.pos + 1
	return &token
}

func (r *reader) peek() *string {
	if r.pos >= len(r.tokens) {
		return nil
	}
	return &r.tokens[r.pos]
}

func tokenize(s string) []string {
	results := make([]string, 0, 1)
	re := regexp.MustCompile(`[\s,]*(~@|[\[\]{}()'` + "`" +
		`~^@]|"(?:\\.|[^\\"])*"?|;.*|[^\s\[\]{}('"` + "`" +
		`,;)]*)`)
	for _, group := range re.FindAllStringSubmatch(s, -1) {
		if (group[1] == "") || (group[1][0] == ';') {
			continue
		}
		results = append(results, group[1])
	}
	return results
}

func readToken(r *reader) (Value, error) {
	token := r.next()
	if token == nil {
		return nil, errors.New("read_atom underflow")
	}
	if match, _ := regexp.MatchString(`^-?[0-9]+$`, *token); match {
		var i int
		var err error
		if i, err = strconv.Atoi(*token); err != nil {
			return nil, errors.New("number parse error")
		}
		return i, nil
	} else if match, _ :=
		regexp.MatchString(`^"(?:\\.|[^\\"])*"$`, *token); match {
		str := (*token)[1 : len(*token)-1]
		return strings.Replace(
			strings.Replace(
				strings.Replace(
					strings.Replace(str, `\\`, "\u029e", -1),
					`\"`, `"`, -1),
				`\n`, "\n", -1),
			"\u029e", "\\", -1), nil
	} else if (*token)[0] == '"' {
		return nil, errors.New("expected '\"', got EOF")
	} else if (*token)[0] == ':' {
		return NewKeyword((*token)[1:len(*token)]), nil
	} else {
		return Symbol(*token), nil
	}
}

func readList(r *reader, start, end string) (Value, error) {
	token := r.next()
	if token == nil {
		log.Fatal("tokens underflow")
	}
	if *token != start {
		return nil, fmt.Errorf("expected '%s'", start)
	}

	var list List

	token = r.peek()
	for ; true; token = r.peek() {
		if token == nil {
			return nil, errors.New("exepected '" + end + "', got EOF")
		}
		if *token == end {
			break
		}
		f, err := read(r)
		if err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	r.next()
	return list, nil
}

func readMap(r *reader) (Value, error) {
	list, err := readList(r, "{", "}")
	if err != nil {
		return nil, err
	}
	return listToMap(list.(List))
}

func read(r *reader) (Value, error) {
	token := r.peek()
	if token == nil {
		log.Fatal("tokens underflow")
	}

	switch *token {
	case "'":
		r.next()
		q, err := read(r)
		if err != nil {
			return nil, err
		}
		return List{Symbol("quote"), q}, nil
	case "`":
		r.next()
		f, err := read(r)
		if err != nil {
			return nil, err
		}
		return List{Symbol("quasiquote"), f}, nil
	case "~":
		r.next()
		f, err := read(r)
		if err != nil {
			return nil, err
		}
		return List{Symbol("unquote"), f}, nil
	case "~@":
		r.next()
		f, err := read(r)
		if err != nil {
			return nil, err
		}
		return List{Symbol("splice-unquote"), f}, nil
	case ")", "}", "]":
		return nil, fmt.Errorf("unexpected '%s'", *token)
	case "(":
		return readList(r, "(", ")")
	case "[":
		return readList(r, "[", "]")
	case "{":
		return readMap(r)
	default:
		return readToken(r)
	}
}

func Read(s string) (Value, error) {
	tokens := tokenize(s)
	if len(tokens) == 0 {
		return nil, nil
	}
	r := &reader{tokens: tokens, pos: 0}
	node, err := read(r)
	if err != nil {
		return nil, err
	}
	if r.pos >= len(r.tokens) {
		return node, nil
	}
	tree := List{Symbol("do"), node}
	for r.pos < len(r.tokens) {
		node, err := read(r)
		if err != nil {
			return nil, err
		}
		tree = append(tree, node)
	}
	return tree, nil
}
