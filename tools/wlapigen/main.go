package main

import (
	_ "embed"
	"encoding/xml"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

var (
	output string
	input  string
	pkg    string
	mode   string
)

//go:embed client.go.tmpl
var clientTmpl string

//go:embed server_wire.go.tmpl
var serverWireTmpl string

//go:embed server_impl.go.tmpl
var serverImplTmpl string

func init() {
	flag.StringVar(&output, "out", ".", "output path")
	flag.StringVar(&input, "in", "api.json", "input path")
	flag.StringVar(&pkg, "pkg", "main", "package name")
	flag.StringVar(&mode, "mode", "client", "mode")
}

func main() {
	flag.Parse()

	var p Protocol

	source, err := os.ReadFile(input)
	if err != nil {
		log.Fatal(err)
	}

	if err := xml.Unmarshal(source, &p); err != nil {
		log.Fatal(err)
	}

	if pkg != "" {
		p.Name = pkg
	}

	switch mode {
	case "client":
		err = execute("client", clientTmpl, fmt.Sprintf("%s/%s.go", output, strings.ToLower(pkg)), p)
	case "server":
		err = execute("server", serverWireTmpl, fmt.Sprintf("%s/%s_wire.go", output, strings.ToLower(pkg)), p)
		if err == nil {
			outfile := fmt.Sprintf("%s/%s_impl.go", output, strings.ToLower(pkg))
			if _, err2 := os.Stat(outfile); err2 != nil {
				err = execute("server", serverImplTmpl, outfile, p)
			} else {
				fmt.Printf("skipping impl\n")
			}
		}
	}

	if err != nil {
		log.Fatal(err)
	}
}

func execute(id, source, output string, ctxt any) error {
	tmpl := template.Must(
		template.New(id).
			Funcs(template.FuncMap{
				"title":  strings.Title,
				"lower":  strings.ToLower,
				"iface":  iface,
				"camel":  camelCase,
				"pascal": pascalCase,
				"type":   toType,
			}).
			Parse(source),
	)

	out := new(strings.Builder)

	if err := tmpl.Execute(out, ctxt); err != nil {
		log.Fatal(err)
	}

	buf, err := format.Source([]byte(out.String()))
	if err != nil {
		log.Fatal(err)
	}

	os.MkdirAll(filepath.Dir(output), 0755)
	return os.WriteFile(
		output,
		buf,
		0644,
	)
}

func iface(s string) string {
	if len(s) == 0 {
		return s
	}
	return pascalCase(strings.TrimPrefix(s, "wl_"))
}

func unSnake(s string) string {
	if len(s) == 0 {
		return s
	}
	v := strings.Split(s, "_")
	for i := range v {
		v[i] = strings.ToUpper(string(v[i][0])) + v[i][1:]
	}
	return strings.Join(v, "")
}

func camelCase(s string) string {
	s = unSnake(s)
	if len(s) == 0 {
		return s
	}
	return string(unicode.ToLower(rune(s[0]))) + s[1:]
}

func pascalCase(s string) string {
	return unSnake(s)
}

func toType(s string) string {
	typeMap := map[string]string{
		"int":    "int32",
		"uint":   "uint32",
		"object": "Proxy",
		"string": "string",
		"fd":     "int32",
		"new_id": "any",
		"fixed":  "float32",
		"array":  "[]byte",
	}

	t, ok := typeMap[s]
	if !ok {
		panic("unknown type name " + s)
	}
	return t
}

func (r Request) ClientBody() string {
	var b strings.Builder

	// 1. new_id object creation
	var newIDs []Argument
	for _, arg := range r.Arguments {
		if arg.Type == "new_id" && arg.Interface != "" {
			newIDs = append(newIDs, arg)
			fmt.Fprintf(
				&b,
				"%s := New%s(i.Context())\n",
				camelCase(arg.Name),
				iface(arg.Interface),
			)
		}
	}

	// base header = 8 bytes, each object/new_id = 4 bytes
	size := 8
	for _, arg := range r.Arguments {
		switch arg.Type {
		case "new_id", "object", "int", "uint", "fixed":
			size += 4
		}
	}

	fmt.Fprintf(&b, "const _reqBufLen = %d\n", size)
	fmt.Fprintf(&b, "var _reqBuf [_reqBufLen]byte\n")

	fmt.Fprintf(&b, "l := 0\n")
	fmt.Fprintf(&b, "PutUint32(_reqBuf[l:4], i.Id())\n")
	fmt.Fprintf(&b, "l += 4\n")
	fmt.Fprintf(
		&b,
		"PutUint32(_reqBuf[l:l+4], uint32(_reqBufLen<<16|opcode&0x0000ffff))\n",
	)
	fmt.Fprintf(&b, "l += 4\n")

	// 5. arguments
	for _, arg := range r.Arguments {
		switch arg.Type {
		case "new_id":
			name := camelCase(arg.Name)
			fmt.Fprintf(&b, "PutUint32(_reqBuf[l:l+4], %s.Id())\n", name)
			fmt.Fprintf(&b, "l += 4\n")
		}
	}

	// 6. send
	fmt.Fprintf(&b, "err := i.Context().WriteMsg(_reqBuf[:], nil)\n")

	// 7. return values
	if len(newIDs) == 1 {
		fmt.Fprintf(&b, "return %s, err\n", camelCase(newIDs[0].Name))
	} else {
		fmt.Fprintf(&b, "return err\n")
	}

	return b.String()
}

func (a Argument) GoType() string {
	if a.Type == "new_id" {
		return pascalCase(a.Interface)
	}
	return toType(a.Type)
}
