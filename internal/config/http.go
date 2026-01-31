package config

import (
	"fmt"
	"net"
)

type HTTP struct {
	Addr string `env:"HTTP_ADDR,default=:8080"`
}

func (h *HTTP) validate() error {
	if h.Addr == "" {
		return fmt.Errorf("HTTP_ADDR must not be empty")
	}

	if _, err := net.ResolveTCPAddr("tcp", h.Addr); err != nil {
		return fmt.Errorf("invalid HTTP_ADDR %q: %w", h.Addr, err)
	}

	return nil
}
