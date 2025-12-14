package main

import "strings"

type Event struct {
	Action    string
	Device    string
	Subsystem string
	Environ   map[string]string
}

func Parse(data []byte) *Event {
	parts := strings.Split(string(data), "\x00")

	ev := &Event{
		Environ: make(map[string]string),
	}

	if len(parts) > 0 {
		if i := strings.Index(parts[0], "@"); i >= 0 {
			ev.Action = parts[0][:i]
			ev.Device = parts[0][i+1:]
		}
	}

	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		if i := strings.Index(p, "="); i >= 0 {
			key := p[:i]
			val := p[i+1:]
			ev.Environ[key] = val
		}
	}

	ev.Subsystem = ev.Environ["SUBSYSTEM"]
	return ev
}
