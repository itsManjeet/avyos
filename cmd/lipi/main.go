package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"avyos.dev/pkg/lipi"
	"avyos.dev/pkg/readline"
)

const (
	InitLipiPath = "/config/init.lipi"
)

var (
	inline   bool
	skipInit bool

	runningProcess *exec.Cmd
)

//go:embed builtin.lipi
var builtinLipi string

func init() {
	flag.BoolVar(&inline, "i", false, "inline script")
	flag.BoolVar(&skipInit, "skip-init", false, "skip init script")
}

func main() {
	flag.Parse()

	registerBusyboxCommands()
	registerSystemCommands()

	lipi.Global.Set("clear", lipi.Process(builtinClear))
	lipi.Eval(builtinLipi)

	if !skipInit {
		source, err := os.ReadFile(InitLipiPath)
		if err != nil {
			fmt.Printf("Failed to load lipi %s: %v\n", InitLipiPath, err)
		}
		lipi.Eval(string(source))
	}

	if flag.NArg() == 0 {
		repl()
		return
	}

	for _, arg := range flag.Args() {
		if inline {
			_, err := lipi.Eval(arg)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v", err)
				os.Exit(1)
			}
		} else {
			data, err := os.ReadFile(arg)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v", err)
				os.Exit(1)
			}

			_, err = lipi.Eval(string(data))
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v", err)
				os.Exit(1)
			}
		}
	}
}

func repl() {
	reader := readline.NewReader("> ")

	installSignalHandler()

	for {
		prompt, err := lipi.Global.Get("PROMPT")
		if err != nil {
			prompt = "> "
		}
		reader.SetPrompt(lipi.ToString(prompt))

		line, err := reader.Readline()
		if err != nil {
			switch err {
			case readline.Interrupt:
				continue
			case readline.EOF:
				break
			}
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v", err)
			continue
		}

		for !isBalanced(line) {
			reader.SetPrompt("...")
			subline, err := reader.Readline()
			if err != nil {
				switch err {
				case readline.Interrupt:
					continue
				case readline.EOF:
					break
				}
				_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v", err)
				continue
			}

			line += " " + subline
		}

		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if line[0] != '(' {
			line = "(" + line + ")"
		}
		result, err := lipi.Eval(line)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}

		if result != nil {
			fmt.Println("::", lipi.ToString(result))
		}
	}
}

func isBalanced(s string) bool {
	brackets := map[rune]rune{
		'(': ')',
		'{': '}',
		'[': ']',
	}
	var stack []rune
	for _, c := range s {
		switch c {
		case '(', '{', '[':
			stack = append(stack, c)
		case ')', '}', ']':
			if len(stack) == 0 {
				// This is error situation but let lipi handle it with
				// detailed error info
				return true
			}
			if stack[len(stack)-1] == brackets[c] {
				stack = stack[:len(stack)-1]
			} else {
				// Again error situation but let lipi handle it
				return true
			}
		}
	}
	return len(stack) == 0
}

func registerBusyboxCommands() {
	output, err := exec.Command("busybox", "--list").CombinedOutput()
	if err != nil {
		return
	}

	for _, b := range strings.Split(string(output), "\n") {
		if b != "" {
			lipi.Global.Set(lipi.Symbol(b), getBusyboxFunc(b))
		}
	}
}

func registerSystemCommands() {
	for _, path := range strings.Split(os.Getenv("PATH"), ":") {
		dir, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, bin := range dir {
			lipi.Global.Set(lipi.Symbol(bin.Name()), getBuiltinFunc(bin.Name()))
		}
	}
}

func getBusyboxFunc(id string) lipi.Process {
	return func(args []lipi.Value) (lipi.Value, error) {
		cmdArgs := make([]string, 0, len(args)+1)
		cmdArgs = append(cmdArgs, id)
		for _, arg := range args {
			cmdArgs = append(cmdArgs, lipi.ToString(arg))
		}
		cmd := exec.Command("busybox", cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		runningProcess = cmd
		err := cmd.Run()
		runningProcess = nil
		return nil, err
	}
}

func getBuiltinFunc(id string) lipi.Process {
	return func(args []lipi.Value) (lipi.Value, error) {
		cmdArgs := make([]string, 0, len(args))
		for _, arg := range args {
			cmdArgs = append(cmdArgs, lipi.ToString(arg))
		}
		cmd := exec.Command(id, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		runningProcess = cmd
		err := cmd.Run()
		runningProcess = nil
		return nil, err
	}
}

func installSignalHandler() {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT)

	go func() {
		for range sigc {
			if runningProcess != nil && runningProcess.Process != nil {
				pgid, err := syscall.Getpgid(runningProcess.Process.Pid)
				if err == nil {
					syscall.Kill(-pgid, syscall.SIGINT)
				}
			}
		}
	}()
}
