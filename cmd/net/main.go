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
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"syscall"
	"unsafe"

	"avyos.dev/pkg/format"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "net - Network interface and routing utilities")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  net <subcommand> [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  interfaces  List network interfaces")
		fmt.Fprintln(os.Stderr, "  address     Show IP addresses")
		fmt.Fprintln(os.Stderr, "  start       Bring interface up")
		fmt.Fprintln(os.Stderr, "  assign      Assign IPv4 address in CIDR form")
		fmt.Fprintln(os.Stderr, "  route       Add default route")
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
		"interfaces": cmdInterfaces,
		"address":    cmdAddress,
		"start":      cmdStart,
		"assign":     cmdAssign,
		"route":      cmdRoute,
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

func cmdInterfaces(args []string) error {
	fs := flag.NewFlagSet("interfaces", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	table := format.NewTable("Name", "MAC Address", "MTU", "Flags")
	for _, iface := range ifaces {
		mac := iface.HardwareAddr.String()
		if mac == "" {
			mac = "(none)"
		}
		table.AddRow(iface.Name, mac, fmt.Sprintf("%d", iface.MTU), iface.Flags.String())
	}
	table.Print()
	return nil
}

func cmdAddress(args []string) error {
	fs := flag.NewFlagSet("address", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	var ifaces []net.Interface
	var err error
	if len(args) > 0 {
		iface, err := net.InterfaceByName(args[0])
		if err != nil {
			return err
		}
		ifaces = []net.Interface{*iface}
	} else {
		ifaces, err = net.Interfaces()
		if err != nil {
			return err
		}
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}

		fmt.Printf("%s:\n", format.Color(format.Bold, iface.Name))
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				fmt.Printf("  %s\n", addr.String())
				continue
			}
			ip := ipnet.IP
			ones, _ := ipnet.Mask.Size()
			ipType := "IPv4"
			if ip.To4() == nil {
				ipType = "IPv6"
			}
			fmt.Printf("  %s: %s/%d\n", ipType, ip.String(), ones)
		}
		fmt.Println()
	}

	return nil
}

func cmdStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) != 1 {
		return fmt.Errorf("usage: net start <interface>")
	}

	iface, err := net.InterfaceByName(args[0])
	if err != nil {
		return err
	}
	return SetLinkUp(iface.Index)
}

func cmdAssign(args []string) error {
	fs := flag.NewFlagSet("assign", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) != 2 {
		return fmt.Errorf("usage: net assign <interface> <ip/prefix>")
	}

	iface, err := net.InterfaceByName(args[0])
	if err != nil {
		return err
	}

	ip, ipnet, err := net.ParseCIDR(args[1])
	if err != nil {
		return err
	}
	prefix, _ := ipnet.Mask.Size()
	return AddIPv4Addr(iface.Index, ip, prefix)
}

func cmdRoute(args []string) error {
	fs := flag.NewFlagSet("route", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) != 2 {
		return fmt.Errorf("usage: net route <interface> <gateway>")
	}

	iface, err := net.InterfaceByName(args[0])
	if err != nil {
		return err
	}

	gw := net.ParseIP(args[1]).To4()
	if gw == nil {
		return fmt.Errorf("invalid gateway IP")
	}
	return AddDefaultRoute(iface.Index, gw)
}

func SetLinkUp(ifindex int) error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	msg := make([]byte, syscall.SizeofIfInfomsg)
	m := (*syscall.IfInfomsg)(unsafe.Pointer(&msg[0]))
	m.Family = syscall.AF_UNSPEC
	m.Index = int32(ifindex)
	m.Flags = syscall.IFF_UP
	m.Change = syscall.IFF_UP

	return nlSend(fd, syscall.RTM_NEWLINK, syscall.NLM_F_REQUEST|syscall.NLM_F_ACK, msg)
}

func AddIPv4Addr(ifindex int, ip net.IP, prefixLen int) error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	msg := make([]byte, syscall.SizeofIfAddrmsg)
	m := (*syscall.IfAddrmsg)(unsafe.Pointer(&msg[0]))
	m.Family = syscall.AF_INET
	m.Prefixlen = uint8(prefixLen)
	m.Index = uint32(ifindex)

	msg = append(msg, nlAttr(syscall.IFA_LOCAL, ip.To4())...)
	msg = append(msg, nlAttr(syscall.IFA_ADDRESS, ip.To4())...)

	return nlSend(fd, syscall.RTM_NEWADDR, syscall.NLM_F_REQUEST|syscall.NLM_F_CREATE|syscall.NLM_F_REPLACE|syscall.NLM_F_ACK, msg)
}

func AddDefaultRoute(ifindex int, gw net.IP) error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	msg := make([]byte, syscall.SizeofRtMsg)
	m := (*syscall.RtMsg)(unsafe.Pointer(&msg[0]))
	m.Family = syscall.AF_INET
	m.Dst_len = 0
	m.Table = syscall.RT_TABLE_MAIN
	m.Protocol = syscall.RTPROT_STATIC
	m.Scope = syscall.RT_SCOPE_UNIVERSE
	m.Type = syscall.RTN_UNICAST

	msg = append(msg, nlAttr(syscall.RTA_GATEWAY, gw.To4())...)
	oif := make([]byte, 4)
	binary.LittleEndian.PutUint32(oif, uint32(ifindex))
	msg = append(msg, nlAttr(syscall.RTA_OIF, oif)...)

	return nlSend(fd, syscall.RTM_NEWROUTE, syscall.NLM_F_REQUEST|syscall.NLM_F_CREATE|syscall.NLM_F_REPLACE|syscall.NLM_F_ACK, msg)
}

func nlAttr(typ uint16, data []byte) []byte {
	l := syscall.NLA_HDRLEN + len(data)
	pad := (4 - (l % 4)) % 4

	b := make([]byte, l+pad)
	*(*uint16)(unsafe.Pointer(&b[0])) = uint16(l)
	*(*uint16)(unsafe.Pointer(&b[2])) = typ
	copy(b[syscall.NLA_HDRLEN:], data)
	return b
}

func nlSend(fd int, typ uint16, flags uint16, data []byte) error {
	hdrLen := syscall.NLMSG_HDRLEN
	msgLen := hdrLen + len(data)

	buf := make([]byte, msgLen)
	hdr := (*syscall.NlMsghdr)(unsafe.Pointer(&buf[0]))
	hdr.Len = uint32(msgLen)
	hdr.Type = typ
	hdr.Flags = flags
	hdr.Seq = 1
	hdr.Pid = 0

	copy(buf[hdrLen:], data)
	_, err := syscall.Write(fd, buf)
	return err
}
