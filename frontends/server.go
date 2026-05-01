package frontends

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	mcnet "github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/frontend"
)

type ServerHandler struct {
	address  netip.AddrPort
	login    string // optional
	motd     string // optional
	warm     chan struct{}
	logger   *slog.Logger
	listener *net.TCPListener
	done     chan error
	closed   bool
}

func (s *ServerHandler) Start() error {
	if s.closed {
		return fmt.Errorf("minecraft server handler is closed")
	}
	if s.listener != nil {
		return nil
	}

	listener, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(s.address))
	if err != nil {
		return err
	}

	s.listener = listener
	s.done = make(chan error, 1)
	go func() {
		s.done <- s.serve(listener)
	}()

	return nil
}

func (s *ServerHandler) Stop() error {
	if s.listener == nil {
		return nil
	}

	err := s.listener.Close()
	s.listener = nil

	serveErr := <-s.done
	s.done = nil
	return errors.Join(err, serveErr)
}

func (s *ServerHandler) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.Stop()
}

func (s *ServerHandler) ShouldWarm() <-chan struct{} {
	return s.warm
}

func (s *ServerHandler) serve(listener *net.TCPListener) error {
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		go s.handleConn(mcnet.WrapConn(conn))
	}
}

func (s *ServerHandler) handleConn(conn *mcnet.Conn) {
	defer conn.Close()

	var (
		p             pk.Packet
		protocol      pk.VarInt
		intention     pk.VarInt
		serverAddress pk.String
		serverPort    pk.UnsignedShort
	)

	if err := conn.ReadPacket(&p); err != nil {
		return
	}
	if err := p.Scan(&protocol, &serverAddress, &serverPort, &intention); err != nil {
		return
	}

	switch int32(intention) {
	case 1:
		s.handleStatus(conn, int32(protocol))
	case 2:
		s.handleLogin(conn)
	}
}

func (s *ServerHandler) handleStatus(conn *mcnet.Conn, clientProtocol int32) {
	var request pk.Packet
	if err := conn.ReadPacket(&request); err != nil {
		return
	}
	if packetid.ServerboundPacketID(request.ID) != packetid.ServerboundStatusRequest {
		return
	}

	response, err := s.statusResponse(clientProtocol)
	if err != nil {
		s.logger.Error("failed to marshal minecraft status response", "err", err)
		return
	}
	if err := conn.WritePacket(pk.Marshal(
		packetid.ClientboundStatusResponse,
		pk.String(response),
	)); err != nil {
		return
	}

	var ping pk.Packet
	if err := conn.ReadPacket(&ping); err != nil {
		return
	}
	if packetid.ServerboundPacketID(ping.ID) != packetid.ServerboundStatusPingRequest {
		return
	}
	_ = conn.WritePacket(pk.Packet{
		ID:   int32(packetid.ClientboundStatusPongResponse),
		Data: ping.Data,
	})
}

func (s *ServerHandler) handleLogin(conn *mcnet.Conn) {
	var request pk.Packet
	if err := conn.ReadPacket(&request); err != nil {
		return
	}
	if packetid.ServerboundPacketID(request.ID) != packetid.ServerboundLoginStart {
		return
	}

	select {
	case s.warm <- struct{}{}:
	default:
	}

	if err := conn.WritePacket(pk.Marshal(
		packetid.ClientboundLoginDisconnect,
		chat.Text(s.login),
	)); err != nil {
		s.logger.Debug("failed to send minecraft login disconnect", "err", err)
	}
}

func (s *ServerHandler) statusResponse(clientProtocol int32) ([]byte, error) {
	description := chat.Text(s.motd)

	resp := struct {
		Version struct {
			Name     string `json:"name"`
			Protocol int32  `json:"protocol"`
		} `json:"version"`
		Players struct {
			Max    int `json:"max"`
			Online int `json:"online"`
		} `json:"players"`
		Description *chat.Message `json:"description"`
	}{}

	resp.Version.Name = "proxygw-minecraft"
	resp.Version.Protocol = clientProtocol
	resp.Description = &description

	return json.Marshal(resp)
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
		logger:  slog.Default().With("handler", "server", "frontend", name),
	}, nil
}
