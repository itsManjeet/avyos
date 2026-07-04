package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type clientActionRequest struct {
	Action  string   `json:"action"`
	Recipes []string `json:"recipes"`
	Force   bool     `json:"force"`
	Push    bool     `json:"push"`
	Message string   `json:"message"`
}

func client(_ *Ignite, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: ignite client <build|fetch|status|workspace|workspace-finish|jobs|logs|cancel|recipes|sources|workspaces> ...")
		return 1
	}
	command := args[0]
	rest := args[1:]
	switch command {
	case "build", "fetch", "status", "workspace", "workspace-finish":
		req := clientActionRequest{Action: command, Recipes: rest, Force: force, Push: workspacePush, Message: workspaceMessage}
		return clientPOSTJSON("/api/actions", req)
	case "jobs", "builds":
		return clientGETJSON("/api/builds")
	case "logs":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: ignite client logs <job-id>")
			return 1
		}
		return clientStreamLogs(rest[0])
	case "cancel":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: ignite client cancel <job-id>")
			return 1
		}
		return clientPOSTJSON("/api/builds/"+rest[0]+"/cancel", map[string]string{})
	case "recipes":
		path := "/api/recipes"
		if len(rest) > 0 {
			path = "/api/recipes/" + rest[0]
		}
		return clientGETJSON(path)
	case "sources":
		return clientGETJSON("/api/sources")
	case "workspaces":
		return clientGETJSON("/api/workspaces")
	default:
		fmt.Fprintln(os.Stderr, "unknown client command:", command)
		return 1
	}
}

func clientGETJSON(path string) int {
	res, err := http.Get(serverURL + path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer res.Body.Close()
	return printHTTPJSON(res)
}

func clientPOSTJSON(path string, body any) int {
	data, _ := json.Marshal(body)
	res, err := http.Post(serverURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer res.Body.Close()
	return printHTTPJSON(res)
}

func printHTTPJSON(res *http.Response) int {
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "%s: %s\n", res.Status, string(data))
		return 1
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		fmt.Println(string(data))
		return 0
	}
	pretty, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(pretty))
	return 0
}

func clientStreamLogs(id string) int {
	res, err := http.Get(serverURL + "/api/builds/" + id + "/logs")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return printHTTPJSON(res)
	}
	buf := make([]byte, 4096)
	var pending string
	for {
		n, err := res.Body.Read(buf)
		if n > 0 {
			pending += string(buf[:n])
			for {
				idx := strings.Index(pending, "\n")
				if idx < 0 {
					break
				}
				line := pending[:idx]
				pending = pending[idx+1:]
				if strings.HasPrefix(line, "data: ") {
					var text string
					if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &text) == nil {
						fmt.Println(text)
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}
