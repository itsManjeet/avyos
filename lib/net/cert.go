/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package net

import (
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
)

func newCAPool() *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if err == nil && pool != nil {
		return pool
	}
	return x509.NewCertPool()
}

func loadCertificate(pool *x509.CertPool, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !pool.AppendCertsFromPEM(data) {
		return fmt.Errorf("failed to parse CA bundle")
	}
	return nil
}

func loadCertificates(pool *x509.CertPool, path string) error {
	certs, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, cert := range certs {
		if cert.IsDir() {
			continue
		}
		if err := loadCertificate(pool, filepath.Join(path, cert.Name())); err != nil {
			continue
		}
	}
	return nil
}
