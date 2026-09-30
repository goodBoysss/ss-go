// Package socks5 implements the CONNECT subset needed by the local client.
package socks5

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

const (
	version5         = 5
	noAuthentication = 0
	commandConnect   = 1
	addressIPv4      = 1
	addressDomain    = 3
	addressIPv6      = 4
)

// Accept negotiates SOCKS5 without authentication and returns a target address.
func Accept(conn net.Conn) (string, error) {
	if err := negotiate(conn); err != nil {
		return "", err
	}
	return readConnectRequest(conn)
}

// ReplySuccess acknowledges the local request; Shadowsocks has no target-connect acknowledgement.
func ReplySuccess(conn net.Conn) error {
	return writeReply(conn, 0)
}

// ReplyFailure reports a SOCKS5 request failure.
func ReplyFailure(conn net.Conn) error {
	return writeReply(conn, 1)
}

// negotiate selects the no-authentication SOCKS5 method.
func negotiate(conn net.Conn) error {
	var header [2]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return fmt.Errorf("read SOCKS5 greeting: %w", err)
	}
	if header[0] != version5 {
		return errors.New("unsupported SOCKS version")
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("read SOCKS5 methods: %w", err)
	}
	found := false
	for _, method := range methods {
		if method == noAuthentication {
			found = true
			break
		}
	}
	if !found {
		_, _ = conn.Write([]byte{version5, 0xff})
		return errors.New("SOCKS5 client does not support no authentication")
	}
	_, err := conn.Write([]byte{version5, noAuthentication})
	return err
}

// readConnectRequest parses a SOCKS5 CONNECT request.
func readConnectRequest(conn net.Conn) (string, error) {
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return "", fmt.Errorf("read SOCKS5 request: %w", err)
	}
	if header[0] != version5 || header[1] != commandConnect || header[2] != 0 {
		_ = writeReply(conn, 7)
		return "", errors.New("only SOCKS5 CONNECT is supported")
	}
	var host string
	switch header[3] {
	case addressIPv4:
		ip := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	case addressIPv6:
		ip := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	case addressDomain:
		var length [1]byte
		if _, err := io.ReadFull(conn, length[:]); err != nil {
			return "", err
		}
		if length[0] == 0 {
			return "", errors.New("empty SOCKS5 domain")
		}
		domain := make([]byte, int(length[0]))
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", err
		}
		host = string(domain)
	default:
		_ = writeReply(conn, 8)
		return "", errors.New("unsupported SOCKS5 address type")
	}
	var port [2]byte
	if _, err := io.ReadFull(conn, port[:]); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, fmt.Sprint(binary.BigEndian.Uint16(port[:]))), nil
}

// writeReply writes a SOCKS5 response with an empty bound address.
func writeReply(conn net.Conn, code byte) error {
	_, err := conn.Write([]byte{version5, code, 0, addressIPv4, 0, 0, 0, 0, 0, 0})
	return err
}
