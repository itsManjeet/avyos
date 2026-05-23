package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Server struct {
	hosts map[string]*http.ServeMux
	certs map[string]*tls.Certificate
}

func NewServer(config *Config) (*Server, error) {
	server := &Server{
		hosts: make(map[string]*http.ServeMux),
	}
	for name, site := range config.Sites {
		mux := http.NewServeMux()
		for path, p := range site.Paths {
			switch p.Type {
			case "file":
				mux.HandleFunc(path, http.FileServer(http.Dir(p.Target)).ServeHTTP)
			case "proxy":
				u, err := url.Parse(p.Target)
				if err != nil {
					return nil, err
				}
				mux.Handle(path, httputil.NewSingleHostReverseProxy(u))
			}
		}
		server.hosts[name] = mux
	}
	return server, nil
}

func (s *Server) LoadCertificates(config *Config) {
	s.certs = make(map[string]*tls.Certificate)
	for name, site := range config.Sites {
		if site.TLS.Cert != "" && site.TLS.Key != "" {
			cert, err := tls.LoadX509KeyPair(site.TLS.Cert, site.TLS.Key)
			if err != nil {
				log.Error("failed to load %s TLS certificate: %v", name, err)
				continue
			}
			s.certs[name] = &cert
		}
	}
}

func (s *Server) TLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if cert, ok := s.certs[chi.ServerName]; ok {
				return cert, nil
			}
			return nil, nil
		},
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for name, mux := range s.hosts {
		if r.Host == name {
			mux.ServeHTTP(w, r)
			return
		}
	}
	http.NotFound(w, r)
}
