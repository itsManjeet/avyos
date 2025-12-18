package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type Specification struct {
	Object struct {
		Id   uint32 `json:"id"`
		Name string `json:"name"`
	} `json:"object"`

	Events []struct {
		Id    uint16 `json:"id"`
		Name  string `json:"name"`
		Input struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"input"`
		Output struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"output"`
	} `json:"events"`
}

type Context struct {
	Package string
	Spec    *Specification
}

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

	source, err := os.ReadFile(input)
	if err != nil {
		log.Fatal(err)
	}

	var spec Specification
	if err := json.Unmarshal(source, &spec); err != nil {
		log.Fatal(err)
	}

	ctxt := Context{
		Package: pkg,
		Spec:    &spec,
	}

	if mode == "client" {
		err = execute("client", clientTmpl, fmt.Sprintf("%s/%s.go", output, strings.ToLower(spec.Object.Name)), ctxt)
	} else if mode == "server" {
		err = execute("server", serverWireTmpl, fmt.Sprintf("%s/%s_wire.go", output, strings.ToLower(spec.Object.Name)), ctxt)
		if err == nil {
			outfile := fmt.Sprintf("%s/%s_impl.go", output, strings.ToLower(spec.Object.Name))
			if _, err2 := os.Stat(outfile); err2 != nil {
				err = execute("server", serverImplTmpl, outfile, ctxt)
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
				"title": strings.Title,
				"lower": strings.ToLower,
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
