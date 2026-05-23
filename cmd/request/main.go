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

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"avyos.dev/pkg/format"
	avynet "avyos.dev/pkg/net"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "request - Network request and connectivity tools")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  request <subcommand> [options] [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  ping     Connectivity probe to host")
		fmt.Fprintln(os.Stderr, "  fetch    HTTP/HTTPS download")
		fmt.Fprintln(os.Stderr, "  listen   TCP listener on port")
		fmt.Fprintln(os.Stderr, "  connect  TCP client to host:port")
		fmt.Fprintln(os.Stderr, "  dns      DNS record lookup")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit Codes:")
		fmt.Fprintln(os.Stderr, "  0  Success")
		fmt.Fprintln(os.Stderr, "  1  Runtime/command error")
		fmt.Fprintln(os.Stderr, "  2  Invalid flags/usage")
	}
}

func main() {
	flag.Parse()
	args := flag.Args()

	commands := map[string]func(args []string) error{
		"ping":    cmdPing,
		"fetch":   cmdFetch,
		"listen":  cmdListen,
		"connect": cmdConnect,
		"dns":     cmdDNS,
	}

	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	cmd, ok := commands[args[0]]
	if !ok {
		format.Error("unknown subcommand: %s", args[0])
		os.Exit(1)
	}

	if err := cmd(args[1:]); err != nil {
		format.Error("%s", err)
		os.Exit(1)
	}
}

func cmdPing(args []string) error {
	fs := flag.NewFlagSet("ping", flag.ContinueOnError)
	count := fs.Int("count", 4, "Number of pings")
	timeout := fs.Int("timeout", 5, "Timeout in seconds")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: request ping <host>")
	}

	host := args[0]
	if *count <= 0 {
		*count = 4
	}
	if *timeout <= 0 {
		*timeout = 5
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("could not resolve host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no IP addresses found for %s", host)
	}

	ip := ips[0]
	fmt.Printf("PING %s (%s)\n\n", host, ip.String())

	successful := 0
	for i := 0; i < *count; i++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip.String(), "80"), time.Duration(*timeout)*time.Second)
		elapsed := time.Since(start)
		if err != nil {
			start = time.Now()
			conn, err = net.DialTimeout("tcp", net.JoinHostPort(ip.String(), "443"), time.Duration(*timeout)*time.Second)
			elapsed = time.Since(start)
		}

		if err != nil {
			fmt.Printf("Request %d: timeout\n", i+1)
		} else {
			conn.Close()
			successful++
			fmt.Printf("Request %d: connected in %v\n", i+1, elapsed.Round(time.Millisecond))
		}

		if i < *count-1 {
			time.Sleep(time.Second)
		}
	}

	fmt.Printf("\n--- %s connectivity statistics ---\n", host)
	fmt.Printf("%d requests, %d successful, %.0f%% packet loss\n",
		*count, successful, float64(*count-successful)/float64(*count)*100)

	return nil
}

func cmdFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	output := fs.String("output", "", "Output file (default: stdout)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: request fetch <url>")
	}

	url := args[0]
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	client := avynet.NewClient()
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var out io.Writer
	if *output != "" {
		file, err := os.Create(*output)
		if err != nil {
			return err
		}
		defer file.Close()
		out = file
	} else {
		out = os.Stdout
	}

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	if *output != "" {
		format.Success("Downloaded %s (%s)", *output, format.Size(written))
	}
	return nil
}

func cmdListen(args []string) error {
	fs := flag.NewFlagSet("listen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: request listen <port>")
	}

	port := args[0]
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("invalid port: %s", port)
	}

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	defer listener.Close()

	fmt.Printf("Listening on port %s...\n", port)
	fmt.Println("Press Ctrl+C to stop")

	for {
		conn, err := listener.Accept()
		if err != nil {
			format.Error("Accept error: %s", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	addr := conn.RemoteAddr().String()
	fmt.Printf("Connection from %s\n", addr)

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				format.Error("Read error: %s", err)
			}
			break
		}
		fmt.Printf("[%s] %s", addr, line)
	}

	fmt.Printf("Connection from %s closed\n", addr)
}

func cmdConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: request connect <host:port>")
	}

	address := args[0]
	if !strings.Contains(address, ":") {
		return fmt.Errorf("address must be in host:port format")
	}

	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Printf("Connected to %s\n", address)
	fmt.Println("Type messages to send (Ctrl+C to exit)")

	go func() {
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					format.Error("Read error: %s", err)
				}
				os.Exit(0)
			}
			fmt.Print(line)
		}
	}()

	stdin := bufio.NewReader(os.Stdin)
	for {
		line, err := stdin.ReadString('\n')
		if err != nil {
			break
		}
		if _, err = conn.Write([]byte(line)); err != nil {
			return err
		}
	}
	return nil
}

func cmdDNS(args []string) error {
	fs := flag.NewFlagSet("dns", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: request dns <hostname>")
	}

	hostname := args[0]

	ips, err := net.LookupIP(hostname)
	if err == nil && len(ips) > 0 {
		fmt.Println("IP Addresses:")
		for _, ip := range ips {
			ipType := "A"
			if ip.To4() == nil {
				ipType = "AAAA"
			}
			fmt.Printf("  %s: %s\n", ipType, ip.String())
		}
		fmt.Println()
	}

	cname, err := net.LookupCNAME(hostname)
	if err == nil && cname != hostname+"." {
		fmt.Printf("CNAME: %s\n\n", cname)
	}

	mxs, err := net.LookupMX(hostname)
	if err == nil && len(mxs) > 0 {
		fmt.Println("MX Records:")
		for _, mx := range mxs {
			fmt.Printf("  %d %s\n", mx.Pref, mx.Host)
		}
		fmt.Println()
	}

	nss, err := net.LookupNS(hostname)
	if err == nil && len(nss) > 0 {
		fmt.Println("Name Servers:")
		for _, ns := range nss {
			fmt.Printf("  %s\n", ns.Host)
		}
		fmt.Println()
	}

	txts, err := net.LookupTXT(hostname)
	if err == nil && len(txts) > 0 {
		fmt.Println("TXT Records:")
		for _, txt := range txts {
			fmt.Printf("  %s\n", txt)
		}
	}

	return nil
}
