package frontends

import (
	"fmt"
	"log/slog"
	"net"
	"net/netip"

	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/frontend"
)

type ServerHandler struct {
	address  netip.AddrPort
	login    string // optional
	motd     string // optional
	warm     chan struct{}
	logger   *slog.Logger
	listener net.Listener
}

func (s ServerHandler) Start() error {
	//TODO Start the server loop
	panic("implement me")
}

func (s ServerHandler) Stop() error {
	//TODO Stop the server loop
	panic("implement me")
}

func (s ServerHandler) Close() error {
	//TODO Stop the server loop and invalidate this handler, making it incapable of starting again
	panic("implement me")
}

func (s ServerHandler) ShouldWarm() <-chan struct{} {
	return s.warm
}

// NewServerHandler creates a Minecraft server pseudo-frontend. It can handle both status requests
// and login attempts.
// Status requests are served with a preconfigured "motd" message.
// Login attempts are served with a preconfigured "login" message, and cause the handler
// to attempt to enqueue a signal onto the "warm" channel.
// All connections are always gracefully closed.
func NewServerHandler(name string, protocol config.Protocol, address netip.AddrPort, options map[string]any) (frontend.Handler, error) {
	if protocol != config.ProtocolTCP {
		return nil, fmt.Errorf("minecraft frontend requires tcp protocol")
	}

	login, _ := options["login"].(string)
	motd, _ := options["motd"].(string)

	return &ServerHandler{
		address: address,
		login:   login,
		motd:    motd,
		warm:    make(chan struct{}),
		logger:  slog.Default().With("component", "handler", "frontend", name),
	}, nil
}
