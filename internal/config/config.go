// Package config defines the standard Shadowsocks JSON configuration.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
)

// Config contains server listeners and local client settings.
type Config struct {
	Mode   string       `json:"mode"`
	Server ServerConfig `json:"server"`
	Client ClientConfig `json:"client"`
	Users  []UserConfig `json:"users"`
}

// ServerConfig contains the default listener and outbound timeout.
type ServerConfig struct {
	Listen             string `json:"listen"`
	DialTimeoutSeconds int    `json:"dial_timeout_seconds"`
}

// ClientConfig contains the local SOCKS5 and remote Shadowsocks settings.
type ClientConfig struct {
	Listen   string `json:"listen"`
	Server   string `json:"server"`
	Password string `json:"password"`
	Method   string `json:"method"`
}

// UserConfig defines one standard Shadowsocks listener account.
type UserConfig struct {
	Listen   string `json:"listen"`
	Password string `json:"password"`
	Method   string `json:"method"`
}

// Load reads JSON, applies an optional mode override, and validates the selected mode.
func Load(path, modeOverride string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return nil, errors.New("config must contain exactly one JSON object")
	}
	if modeOverride != "" {
		cfg.Mode = modeOverride
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks standard Shadowsocks methods and listener account settings.
func (c *Config) Validate() error {
	if c.Mode != "server" && c.Mode != "client" {
		return errors.New("mode must be server or client")
	}
	if c.Mode == "server" {
		if c.Server.DialTimeoutSeconds < 0 || c.Server.DialTimeoutSeconds > 300 {
			return errors.New("server.dial_timeout_seconds must be between 0 and 300")
		}
		if len(c.Users) == 0 {
			return errors.New("server users must contain at least one account")
		}
		seen := make(map[string]struct{}, len(c.Users))
		for index := range c.Users {
			user := &c.Users[index]
			if user.Listen == "" {
				if len(c.Users) != 1 || c.Server.Listen == "" {
					return fmt.Errorf("users[%d].listen is required when multiple users are configured", index)
				}
				user.Listen = c.Server.Listen
			}
			if user.Password == "" {
				return fmt.Errorf("users[%d].password is required", index)
			}
			if err := validateAddress(user.Listen, false); err != nil {
				return fmt.Errorf("users[%d].listen: %w", index, err)
			}
			user.Method = normalizeMethod(user.Method)
			if !supportedMethod(user.Method) {
				return fmt.Errorf("users[%d].method %q is unsupported", index, user.Method)
			}
			if _, exists := seen[user.Listen]; exists {
				return fmt.Errorf("duplicate listener %q", user.Listen)
			}
			seen[user.Listen] = struct{}{}
		}
		return nil
	}
	if c.Client.Listen == "" || c.Client.Server == "" {
		return errors.New("client.listen and client.server are required")
	}
	if c.Client.Password == "" {
		return errors.New("client.password is required")
	}
	if err := validateAddress(c.Client.Listen, false); err != nil {
		return fmt.Errorf("client.listen: %w", err)
	}
	if err := validateAddress(c.Client.Server, true); err != nil {
		return fmt.Errorf("client.server: %w", err)
	}
	c.Client.Method = normalizeMethod(c.Client.Method)
	if !supportedMethod(c.Client.Method) {
		return fmt.Errorf("client.method %q is unsupported", c.Client.Method)
	}
	return nil
}

// validateAddress requires a numeric TCP port and optionally a remote hostname.
func validateAddress(address string, requireHost bool) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return errors.New("expected host:port or :port")
	}
	if requireHost && host == "" {
		return errors.New("server hostname is required")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

// normalizeMethod applies the default modern Shadowsocks cipher name.
func normalizeMethod(method string) string {
	if method == "" {
		return "aes-256-gcm"
	}
	return method
}

// supportedMethod reports whether the cipher is supported by go-shadowsocks2.
func supportedMethod(method string) bool {
	switch method {
	case "aes-128-gcm", "aes-256-gcm", "chacha20-ietf-poly1305":
		return true
	default:
		return false
	}
}
