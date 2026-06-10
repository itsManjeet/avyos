package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Container struct {
	environ      []string
	binds        [][2]string
	capabilities []string
	hostRoot     string
	baseDir      string
	name         string
	logger       io.Writer
}

func (c Container) Args() []string {
	args := []string{"/bin/bwrap", "--bind", c.hostRoot, "/", "--proc", "/proc", "--dev", "/dev", "--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf", "--unshare-all", "--share-net", "--uid", "0", "--gid", "0", "--die-with-parent"}
	for _, bind := range c.binds {
		args = append(args, "--bind", bind[1], bind[0])
	}
	for _, cap := range c.capabilities {
		args = append(args, "--cap-add", cap)
	}
	for _, env := range c.environ {
		parts := strings.SplitN(env, "=", 2)
		value := ""
		if len(parts) == 2 {
			value = parts[1]
		}
		args = append(args, "--setenv", parts[0], value)
	}
	return args
}

type Executor struct {
	args        []string
	path        string
	environ     []string
	logger      io.Writer
	silent      bool
	interactive bool
}

func NewExecutor(binary string) *Executor {
	return &Executor{args: []string{binary}}
}

func (e *Executor) Arg(value string) *Executor {
	e.args = append(e.args, value)
	return e
}

func (e *Executor) Path(path string) *Executor {
	e.path = path
	return e
}

func (e *Executor) Environ(env ...string) *Executor {
	e.environ = append(e.environ, env...)
	return e
}

func (e *Executor) Silent() *Executor {
	e.silent = true
	return e
}

func (e *Executor) Interactive() *Executor {
	e.interactive = true
	return e
}

func (e *Executor) Container(c *Container) *Executor {
	if c == nil {
		return e
	}
	path := e.path
	if path == "" {
		path = "/"
	}
	args := append(c.Args(), "--chdir", path)
	e.args = append(args, e.args...)
	e.path = ""
	e.logger = c.logger
	return e
}

func (e *Executor) command() *exec.Cmd {
	cmd := exec.Command(e.args[0], e.args[1:]...)
	cmd.Env = e.environ
	if e.path != "" {
		cmd.Dir = e.path
	}
	return cmd
}

func (e *Executor) Run() int {
	cmd := e.command()
	if e.interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		out := io.Discard
		if !e.silent {
			out = os.Stdout
		}
		if e.logger != nil {
			out = io.MultiWriter(out, e.logger)
		}
		cmd.Stdout = out
		cmd.Stderr = out
	}
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		return 127
	}
	return 0
}

func (e *Executor) Output() (int, string) {
	cmd := e.command()
	var out bytes.Buffer
	writer := io.Writer(&out)
	if e.logger != nil {
		writer = io.MultiWriter(&out, e.logger)
	}
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode(), strings.TrimSuffix(out.String(), "\n")
		}
		return 127, strings.TrimSuffix(out.String(), "\n")
	}
	return 0, strings.TrimSuffix(out.String(), "\n")
}

func (e *Executor) DumpCommand(w io.Writer) {
	path := e.path
	if path == "" {
		path = "."
	}
	fmt.Fprintf(w, "COMMAND  : %s \npath     : %s\n", strings.Join(e.args, " "), path)
}

func (e *Executor) Execute() error {
	if !e.silent {
		if e.logger != nil {
			e.DumpCommand(e.logger)
		} else {
			e.DumpCommand(os.Stdout)
		}
	}
	if status := e.Run(); status != 0 {
		e.DumpCommand(os.Stderr)
		if e.logger != nil {
			e.DumpCommand(e.logger)
		}
		return fmt.Errorf("failed to execute command : exit code %d", status)
	}
	return nil
}
