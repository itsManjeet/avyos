package main

import (
	"os"
	"path/filepath"
	"strings"

	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/ini"
)

type Path struct {
	Type   string
	Target string
}

type Site struct {
	Name  string
	TLS   *TLS
	Paths map[string]Path
}

type TLS struct {
	Cert string
	Key  string
}

type Config struct {
	Listen    string
	ListenTLS string
	Sites     map[string]Site
}

func ParseConfig(configfile string) (*Config, error) {
	c, err := ini.ParseFile(configfile)
	if err != nil {
		return nil, err
	}

	config := Config{
		Sites: map[string]Site{},
	}
	var ok bool
	config.Listen, ok = c.Get("", "listen")
	if !ok {
		config.Listen = ":80"
	}

	config.ListenTLS, ok = c.Get("", "listen-tls")
	if !ok {
		config.ListenTLS = ":443"
	}

	configs, err := os.ReadDir(fs.Resolve("config:http/"))
	if err != nil {
		return &config, nil
	}

	for _, c := range configs {
		if c.IsDir() || filepath.Ext(c.Name()) != ".conf" {
			continue
		}
		s := fs.Resolve("config:http/%s", filepath.Base(c.Name()))
		sc, err := ini.ParseFile(s)
		if err != nil {
			log.Error("failed to read %s config: %v", s, err)
			continue
		}
		var site Site
		if enabled, ok := sc.Get("", "enabled"); ok && (enabled != "true" && enabled != "yes") {
			continue
		}
		site.Name, ok = sc.Get("", "name")
		if !ok {
			log.Error("failed to get name for site %s", s)
			continue
		}
		site.Paths = map[string]Path{}
		for path, _ := range sc.Sections {
			if !strings.HasPrefix(path, "/") {
				continue
			}
			_type, ok := sc.Get(path, "type")
			if !ok {
				log.Error("no type specified for %s", s)
				continue
			}
			target, ok := sc.Get(path, "target")
			if !ok {
				log.Error("no target specified for %s", s)
				continue
			}
			site.Paths[path] = Path{Type: _type, Target: target}
		}

		if _, ok := sc.Sections["tls"]; ok {
			tls := TLS{}
			tls.Cert, ok = sc.Get("tls", "cert")
			if !ok {
				log.Error("no cert specified for tls")
				continue
			}
			tls.Key, ok = sc.Get("tls", "key")
			if !ok {
				log.Error("no key specified for tls")
				continue
			}
			site.TLS = &tls
		}

		config.Sites[site.Name] = site
	}
	return &config, nil
}
