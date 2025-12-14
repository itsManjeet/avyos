package ensure

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func Output[T comparable](actual, expected T, format string, args ...interface{}) {
	if actual != expected {
		log.Fatalf(format, args...)
	}
}

func Foreach[T any](l []T, f func(l T) error) error {
	for _, v := range l {
		if err := f(v); err != nil {
			return err
		}
	}
	return nil
}

func Success(err error, format string, args ...interface{}) {
	if err != nil {
		args = append(args, err)
		log.Fatalf(format+": %v", args...)
	}
}

func Target(t string, f func() error, depends ...string) error {
	if tInfo, err := os.Stat(t); err == nil {
		for _, dep := range depends {
			depInfo, err := os.Stat(dep)
			if err != nil {
				return fmt.Errorf("could not stat dependency %q: %w", dep, err)
			}
			if depInfo.ModTime().After(tInfo.ModTime()) {
				log.Printf("Dependency %q is newer than target %q\n", dep, t)
				goto BUILD
			}
		}
		return nil
	}

BUILD:
	log.Println("TARGET:", t)
	return f()
}

func Cmd(bin string, args ...string) func() error {
	return func() error {
		cmd := exec.Command(bin, args...)
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		log.Println("COMMAND:", cmd.Args)
		return cmd.Run()
	}
}

func Script(fs ...func() error) func() error {
	return func() error {
		for _, f := range fs {
			if err := f(); err != nil {
				return err
			}
		}
		return nil
	}
}
