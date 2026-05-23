package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"avyos.dev/pkg/config"
)

// Template-data types (used in TmplData and templates)

type Field struct {
	Name string
	Type string
}

type TypeDef struct {
	Name   string
	Fields []Field
}

// Protocol config schema
// Fields are named blocks: fields { Name { type = T } }
// All named blocks (types, objects, requests, events, properties, fields) are
// sorted alphabetically for deterministic opcode and struct-field assignment.

type cfgField struct {
	Type string `config:"type"`
}

type cfgTypeDef struct {
	Fields map[string]cfgField `config:"fields"`
}

type cfgRequest struct {
	In  []string `config:"in"`
	Out []string `config:"out"`
}

type cfgEvent struct {
	Args []string `config:"args"`
}

type cfgProperty struct {
	Type     string `config:"type"`
	ReadOnly bool   `config:"read_only"`
}

type cfgObject struct {
	Requests   map[string]cfgRequest  `config:"requests"`
	Events     map[string]cfgEvent    `config:"events"`
	Properties map[string]cfgProperty `config:"properties"`
}

type cfgProtocol struct {
	ID      string                `config:"id"`
	Types   map[string]cfgTypeDef `config:"types"`
	Objects map[string]cfgObject  `config:"objects"`
}

// Type mapping
var primitives = map[string]string{
	"uint8": "uint8", "uint16": "uint16", "uint32": "uint32", "uint64": "uint64",
	"int8": "int8", "int16": "int16", "int32": "int32", "int64": "int64",
	"float32": "float32", "float64": "float64",
	"bool": "uint8",
}

func goType(t string) string {
	if g, ok := primitives[t]; ok {
		return g
	}
	return t
}

func unexport(name string) string {
	if len(name) == 0 {
		return name
	}
	return strings.ToLower(name[:1]) + name[1:]
}

// Template data — pre-resolved types ready for templates

type TmplField struct {
	Name string
	Type string
}

type TmplComposite struct {
	Name   string
	Fields []TmplField
}

type TmplOpcode struct {
	Name   string
	Opcode uint16
}

type TmplRequest struct {
	Obj     string
	Name    string
	InType  string
	OutType string
	HasIn   bool
	HasOut  bool
	OpName  string
}

type TmplEvent struct {
	Obj      string
	Name     string
	ArgsType string
	HasArgs  bool
	OpName   string
	Field    string
}

type TmplProperty struct {
	Obj        string
	Name       string
	Type       string
	ReadOnly   bool
	OpGet      string
	OpSet      string
	OpChanged  string
	Field      string
}

type TmplObject struct {
	Name               string
	ID                 uint32
	Requests           []TmplRequest
	Events             []TmplEvent
	Properties         []TmplProperty
	WritableProperties []TmplProperty
	HasCallbacks       bool
}

type TmplData struct {
	ID       string
	Pkg      string
	SutraPkg string
	FsPkg    string
	Types    []TypeDef
	Composites []TmplComposite
	Opcodes    []TmplOpcode
	Objects    []TmplObject
}

func resolveArgs(args []string, structName string) (typeName string, fields []TmplField) {
	switch len(args) {
	case 0:
		return "", nil
	case 1:
		return goType(args[0]), nil
	default:
		f := make([]TmplField, len(args))
		for i, a := range args {
			f[i] = TmplField{Name: fmt.Sprintf("Arg%d", i), Type: goType(a)}
		}
		return structName, f
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func buildTmplData(proto *cfgProtocol, pkg, mod string) (TmplData, error) {
	d := TmplData{
		ID:       proto.ID,
		Pkg:      pkg,
		SutraPkg: mod + "/pkg/sutra",
		FsPkg:    mod + "/pkg/fs",
	}

	// Types — sorted for stable output
	for _, typeName := range sortedKeys(proto.Types) {
		t := proto.Types[typeName]
		td := TypeDef{Name: typeName}
		for _, fieldName := range sortedKeys(t.Fields) {
			td.Fields = append(td.Fields, Field{Name: fieldName, Type: goType(t.Fields[fieldName].Type)})
		}
		d.Types = append(d.Types, td)
	}

	op := uint16(1)
	objID := uint32(0)

	for _, objName := range sortedKeys(proto.Objects) {
		obj := proto.Objects[objName]
		tobj := TmplObject{Name: objName, ID: objID}
		objID++

		// Requests — sorted for stable opcodes
		for _, rName := range sortedKeys(obj.Requests) {
			r := obj.Requests[rName]
			prefix := objName + rName
			inType, inFields := resolveArgs(r.In, prefix+"In")
			outType, outFields := resolveArgs(r.Out, prefix+"Out")
			if inFields != nil {
				d.Composites = append(d.Composites, TmplComposite{Name: prefix + "In", Fields: inFields})
			}
			if outFields != nil {
				d.Composites = append(d.Composites, TmplComposite{Name: prefix + "Out", Fields: outFields})
			}
			opName := "Op" + objName + rName
			d.Opcodes = append(d.Opcodes, TmplOpcode{opName, op})
			tobj.Requests = append(tobj.Requests, TmplRequest{
				Obj: objName, Name: rName,
				InType: inType, OutType: outType,
				HasIn: inType != "", HasOut: outType != "",
				OpName: opName,
			})
			op++
		}

		// Events — sorted for stable opcodes
		for _, eName := range sortedKeys(obj.Events) {
			e := obj.Events[eName]
			prefix := objName + eName
			argsType, argsFields := resolveArgs(e.Args, prefix+"Args")
			if argsFields != nil {
				d.Composites = append(d.Composites, TmplComposite{Name: prefix + "Args", Fields: argsFields})
			}
			opName := "Op" + objName + eName
			d.Opcodes = append(d.Opcodes, TmplOpcode{opName, op})
			tobj.Events = append(tobj.Events, TmplEvent{
				Obj: objName, Name: eName,
				ArgsType: argsType, HasArgs: argsType != "",
				OpName: opName, Field: unexport(eName),
			})
			op++
		}

		// Properties — sorted for stable opcodes
		for _, pName := range sortedKeys(obj.Properties) {
			p := obj.Properties[pName]
			gt := goType(p.Type)
			opGet := "Op" + objName + pName + "Get"
			d.Opcodes = append(d.Opcodes, TmplOpcode{opGet, op})
			op++

			tp := TmplProperty{
				Obj: objName, Name: pName, Type: gt,
				ReadOnly: p.ReadOnly, OpGet: opGet,
				Field: unexport(pName) + "Changed",
			}
			if !p.ReadOnly {
				tp.OpSet = "Op" + objName + pName + "Set"
				d.Opcodes = append(d.Opcodes, TmplOpcode{tp.OpSet, op})
				op++
				tp.OpChanged = "Op" + objName + pName + "Changed"
				d.Opcodes = append(d.Opcodes, TmplOpcode{tp.OpChanged, op})
				op++
				tobj.WritableProperties = append(tobj.WritableProperties, tp)
			}
			tobj.Properties = append(tobj.Properties, tp)
		}

		tobj.HasCallbacks = len(tobj.Events) > 0 || len(tobj.WritableProperties) > 0
		d.Objects = append(d.Objects, tobj)
	}

	return d, nil
}

// Template helpers

var funcs = template.FuncMap{
	"handlerSig": func(r TmplRequest) string {
		var b strings.Builder
		b.WriteString(r.Name)
		b.WriteString("(object uint32")
		if r.HasIn {
			fmt.Fprintf(&b, ", in %s", r.InType)
		}
		b.WriteString(")")
		if r.HasOut {
			fmt.Fprintf(&b, " (%s, error)", r.OutType)
		} else {
			b.WriteString(" error")
		}
		return b.String()
	},
	"callArgs": func(r TmplRequest) string {
		if r.HasIn {
			return "tx.Object, in"
		}
		return "tx.Object"
	},
}

func execTmpl(name, text string, data any) ([]byte, error) {
	t, err := template.New(name).Funcs(funcs).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("exec template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

func writeFormatted(path, name, tmplText string, data any) error {
	raw, err := execTmpl(name, tmplText, data)
	if err != nil {
		return err
	}
	src, err := format.Source(raw)
	if err != nil {
		os.WriteFile(path+".broken", raw, 0644)
		return fmt.Errorf("gofmt %s: %w", path, err)
	}
	return os.WriteFile(path, src, 0644)
}

// Templates

var typesTmpl = `// Code generated by genapi from {{.ID}}. DO NOT EDIT.
package {{.Pkg}}

{{range .Types}}
type {{.Name}} struct {
{{- range .Fields}}
	{{.Name}} {{.Type}}
{{- end}}
}
{{end}}

{{range .Composites}}
type {{.Name}} struct {
{{- range .Fields}}
	{{.Name}} {{.Type}}
{{- end}}
}
{{end}}

const (
{{- range .Opcodes}}
	{{.Name}} uint16 = {{.Opcode}}
{{- end}}
)
`

var serverTmpl = `// Code generated by genapi from {{.ID}}. DO NOT EDIT.
package {{.Pkg}}

import (
	"fmt"
	"net"
	"{{.FsPkg}}"
	"{{.SutraPkg}}"
)

{{range .Objects}}
type {{.Name}}Handler interface {
{{- range .Requests}}
	{{handlerSig .}}
{{- end}}
{{- range .Properties}}
	Get{{.Name}}(object uint32) ({{.Type}}, error)
{{- if not .ReadOnly}}
	Set{{.Name}}(object uint32, val {{.Type}}) error
{{- end}}
{{- end}}
}
{{end}}

type Handlers struct {
{{- range .Objects}}
	{{.Name}} {{.Name}}Handler
{{- end}}
}

func Dispatch(h Handlers, conn *sutra.Conn, tx sutra.Transaction) error {
	switch tx.Event {
{{- range .Objects}}
{{- $obj := .Name}}
{{- range .Requests}}
	case {{.OpName}}:
{{- if .HasIn}}
		in, err := sutra.UnmarshalPayload[{{.InType}}](tx.Payload)
		if err != nil {
			return fmt.Errorf("decode {{$obj}}.{{.Name}}: %w", err)
		}
{{- end}}
{{- if .HasOut}}
		out, err := h.{{$obj}}.{{.Name}}({{callArgs .}})
		if err != nil { return err }
		payload, err := sutra.MarshalPayload(out)
		if err != nil {
			return fmt.Errorf("encode {{$obj}}.{{.Name}}: %w", err)
		}
		return conn.Send(sutra.Transaction{Object: tx.Object, Event: tx.Event, Payload: payload})
{{- else}}
		if err := h.{{$obj}}.{{.Name}}({{callArgs .}}); err != nil { return err }
		return conn.Send(sutra.Transaction{Object: tx.Object, Event: tx.Event})
{{- end}}
{{- end}}
{{- range .Properties}}
	case {{.OpGet}}:
		val, err := h.{{$obj}}.Get{{.Name}}(tx.Object)
		if err != nil { return err }
		payload, err := sutra.MarshalPayload(val)
		if err != nil {
			return fmt.Errorf("encode {{$obj}}.{{.Name}}: %w", err)
		}
		return conn.Send(sutra.Transaction{Object: tx.Object, Event: tx.Event, Payload: payload})
{{- if not .ReadOnly}}
	case {{.OpSet}}:
		val, err := sutra.UnmarshalPayload[{{.Type}}](tx.Payload)
		if err != nil {
			return fmt.Errorf("decode {{$obj}}.{{.Name}}: %w", err)
		}
		if err := h.{{$obj}}.Set{{.Name}}(tx.Object, val); err != nil { return err }
		return conn.Send(sutra.Transaction{Object: tx.Object, Event: tx.Event})
{{- end}}
{{- end}}
{{- end}}
	default:
		return fmt.Errorf("unknown opcode: %d", tx.Event)
	}
}

{{range .Objects}}
{{- $obj := .Name}}
{{- range .Events}}
func Send{{$obj}}{{.Name}}(conn *sutra.Conn, object uint32{{if .HasArgs}}, args {{.ArgsType}}{{end}}) error {
{{- if .HasArgs}}
	payload, err := sutra.MarshalPayload(args)
	if err != nil { return err }
	return conn.Send(sutra.Transaction{Object: object, Event: {{.OpName}}, Payload: payload})
{{- else}}
	return conn.Send(sutra.Transaction{Object: object, Event: {{.OpName}}})
{{- end}}
}
{{end}}
{{- range .WritableProperties}}
func Notify{{$obj}}{{.Name}}Changed(conn *sutra.Conn, object uint32, val {{.Type}}) error {
	payload, err := sutra.MarshalPayload(val)
	if err != nil { return err }
	return conn.Send(sutra.Transaction{Object: object, Event: {{.OpChanged}}, Payload: payload})
}
{{end}}
{{- end}}

// Server listens on the {{.ID}} Unix socket and dispatches incoming requests.
type Server struct {
	Handlers
	ln net.Listener
}

// Listen creates the {{.ID}} Unix socket and returns a Server ready to serve.
func Listen() (*Server, error) {
	ln, err := net.Listen("unix", fs.Resolve("service:{{.ID}}"))
	if err != nil {
		return nil, err
	}
	return &Server{ln: ln}, nil
}

// Serve accepts connections and dispatches each transaction in a goroutine.
// It blocks until Close is called or a fatal accept error occurs.
func (s *Server) Serve() error {
	for {
		nc, err := s.ln.Accept()
		if err != nil {
			return err
		}
		go s.serveConn(nc)
	}
}

func (s *Server) serveConn(nc net.Conn) {
	conn := sutra.NewConn(nc)
	defer conn.Close()
	for {
		tx, err := conn.Recv()
		if err != nil {
			return
		}
		if err := Dispatch(s.Handlers, conn, tx); err != nil {
			return
		}
	}
}

// Close stops the listener.
func (s *Server) Close() error {
	return s.ln.Close()
}
`

var clientTmpl = `// Code generated by genapi from {{.ID}}. DO NOT EDIT.
package {{.Pkg}}

import (
	"net"
	"{{.FsPkg}}"
	"{{.SutraPkg}}"
)

// Client holds connections to all objects in the {{.ID}} service.
type Client struct {
	conn *sutra.Conn
{{- range .Objects}}
	{{.Name}} *{{.Name}}Client
{{- end}}
}

// Connect dials the {{.ID}} service over a Unix socket.
func Connect() (*Client, error) {
	nc, err := net.Dial("unix", fs.Resolve("service:{{.ID}}"))
	if err != nil {
		return nil, err
	}
	conn := sutra.NewConn(nc)
	return &Client{
		conn: conn,
{{- range .Objects}}
		{{.Name}}: New{{.Name}}Client(conn, {{.ID}}),
{{- end}}
	}, nil
}

// HandleEvent dispatches an incoming event to the appropriate object client.
func (cl *Client) HandleEvent(tx sutra.Transaction) error {
	return sutra.DispatchEvent(tx{{range .Objects}}{{if .HasCallbacks}}, cl.{{.Name}}{{end}}{{end}})
}

// Close closes the underlying connection.
func (cl *Client) Close() error {
	return cl.conn.Close()
}

{{range .Objects}}
{{- $obj := .Name}}

type {{$obj}}Client struct {
	Conn   *sutra.Conn
	Object uint32
{{- range .Events}}
	{{.Field}} func({{if .HasArgs}}args {{.ArgsType}}{{end}})
{{- end}}
{{- range .WritableProperties}}
	{{.Field}} func(val {{.Type}})
{{- end}}
}

func New{{$obj}}Client(conn *sutra.Conn, object uint32) *{{$obj}}Client {
	return &{{$obj}}Client{Conn: conn, Object: object}
}

{{range .Events}}
func (c *{{$obj}}Client) On{{.Name}}(fn func({{if .HasArgs}}args {{.ArgsType}}{{end}})) {
	c.{{.Field}} = fn
}
{{end}}

{{range .WritableProperties}}
func (c *{{$obj}}Client) On{{.Name}}Changed(fn func(val {{.Type}})) {
	c.{{.Field}} = fn
}
{{end}}

{{if .HasCallbacks}}
func (c *{{$obj}}Client) HandleEvent(tx sutra.Transaction) (bool, error) {
	if tx.Object != c.Object { return false, nil }
	switch tx.Event {
{{- range .Events}}
	case {{.OpName}}:
		if c.{{.Field}} != nil {
{{- if .HasArgs}}
			args, err := sutra.UnmarshalPayload[{{.ArgsType}}](tx.Payload)
			if err != nil { return true, err }
			c.{{.Field}}(args)
{{- else}}
			c.{{.Field}}()
{{- end}}
		}
		return true, nil
{{- end}}
{{- range .WritableProperties}}
	case {{.OpChanged}}:
		if c.{{.Field}} != nil {
			val, err := sutra.UnmarshalPayload[{{.Type}}](tx.Payload)
			if err != nil { return true, err }
			c.{{.Field}}(val)
		}
		return true, nil
{{- end}}
	}
	return false, nil
}
{{end}}

{{range .Requests}}
{{if .HasOut}}
func (c *{{$obj}}Client) {{.Name}}({{if .HasIn}}in {{.InType}}{{end}}) ({{.OutType}}, error) {
	var zero {{.OutType}}
{{- if .HasIn}}
	payload, err := sutra.MarshalPayload(in)
	if err != nil { return zero, err }
	resp, err := c.Conn.SendRecv(sutra.Transaction{Object: c.Object, Event: {{.OpName}}, Payload: payload})
{{- else}}
	resp, err := c.Conn.SendRecv(sutra.Transaction{Object: c.Object, Event: {{.OpName}}})
{{- end}}
	if err != nil { return zero, err }
	return sutra.UnmarshalPayload[{{.OutType}}](resp.Payload)
}
{{else}}
func (c *{{$obj}}Client) {{.Name}}({{if .HasIn}}in {{.InType}}{{end}}) error {
{{- if .HasIn}}
	payload, err := sutra.MarshalPayload(in)
	if err != nil { return err }
	_, err = c.Conn.SendRecv(sutra.Transaction{Object: c.Object, Event: {{.OpName}}, Payload: payload})
{{- else}}
	_, err := c.Conn.SendRecv(sutra.Transaction{Object: c.Object, Event: {{.OpName}}})
{{- end}}
	return err
}
{{end}}
{{- end}}

{{range .Properties}}
func (c *{{$obj}}Client) Get{{.Name}}() ({{.Type}}, error) {
	var zero {{.Type}}
	resp, err := c.Conn.SendRecv(sutra.Transaction{Object: c.Object, Event: {{.OpGet}}})
	if err != nil { return zero, err }
	return sutra.UnmarshalPayload[{{.Type}}](resp.Payload)
}
{{if not .ReadOnly}}
func (c *{{$obj}}Client) Set{{.Name}}(val {{.Type}}) error {
	payload, err := sutra.MarshalPayload(val)
	if err != nil { return err }
	_, err = c.Conn.SendRecv(sutra.Transaction{Object: c.Object, Event: {{.OpSet}}, Payload: payload})
	return err
}
{{end}}
{{- end}}
{{- end}}
`

// Utilities

func findModule(startDir string) string {
	dir, _ := filepath.Abs(startDir)
	for dir != "/" && dir != "." {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module "))
				}
			}
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", path, err)
	}
	return data
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "genapi: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	apiPath := flag.String("api", "", "path to protocol JSON file")
	outDir := flag.String("out", ".", "output directory")
	server := flag.Bool("server", false, "generate server-side code")
	client := flag.Bool("client", false, "generate client-side code")
	pkg := flag.String("pkg", "", "package name (default: basename of -out)")
	flag.Parse()

	if *apiPath == "" {
		fmt.Fprintln(os.Stderr, "usage: genapi -api <protocol.json> [-server] [-client] [-out <dir>] [-pkg <name>]")
		os.Exit(1)
	}

	var proto cfgProtocol
	if err := config.Unmarshal(mustRead(*apiPath), &proto); err != nil {
		fatal("parse %s: %v", *apiPath, err)
	}

	if *pkg == "" {
		*pkg = filepath.Base(*outDir)
	}

	cwd, _ := os.Getwd()
	mod := findModule(cwd)
	if mod == "" {
		fatal("could not find go.mod")
	}

	os.MkdirAll(*outDir, 0755)

	d, err := buildTmplData(&proto, *pkg, mod)
	if err != nil {
		fatal("build: %v", err)
	}

	if err := writeFormatted(filepath.Join(*outDir, "types.go"), "types", typesTmpl, d); err != nil {
		fatal("types: %v", err)
	}

	if *server {
		if err := writeFormatted(filepath.Join(*outDir, "server.go"), "server", serverTmpl, d); err != nil {
			fatal("server: %v", err)
		}
	}

	if *client {
		if err := writeFormatted(filepath.Join(*outDir, "client.go"), "client", clientTmpl, d); err != nil {
			fatal("client: %v", err)
		}
	}

	fmt.Printf("genapi: generated %s", *outDir)
	if *server {
		fmt.Print(" [server]")
	}
	if *client {
		fmt.Print(" [client]")
	}
	fmt.Println()
}
