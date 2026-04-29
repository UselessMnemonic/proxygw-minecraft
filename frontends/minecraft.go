package frontends

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"time"

	mcpacket "github.com/Tnze/go-mc/net/packet"
	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/frontend"
)

const (
	defaultMessage      = "Server is starting, please try again soon."
	defaultStatusText   = "Proxy Gateway"
	minecraftReadWindow = 5 * time.Second
)

type Handler struct {
	address netip.AddrPort
	message string
	status  string
	warm    chan struct{}

	logger   *slog.Logger
	listener net.Listener
	wg       sync.WaitGroup
}

// Start opens the Minecraft TCP listener.
func (h *Handler) Start() error {
	if h.listener != nil {
		h.logger.Info("minecraft frontend already started", "listen", h.address.String())
		return nil
	}

	listener, err := net.Listen("tcp", h.address.String())
	if err != nil {
		h.logger.Error("minecraft frontend listen failed", "listen", h.address.String(), "err", err)
		return err
	}

	h.listener = listener
	h.wg.Go(func() {
		h.accept(listener)
	})
	h.logger.Info("minecraft frontend started", "listen", h.address.String())
	return nil
}

// Stop closes the listener and waits for in-flight handlers to exit.
func (h *Handler) Stop() error {
	if h.listener == nil {
		h.logger.Info("minecraft frontend already stopped")
		return nil
	}

	listener := h.listener
	h.listener = nil

	err := listener.Close()
	h.wg.Wait()
	h.logger.Info("minecraft frontend stopped")
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

// Close permanently tears down the handler.
func (h *Handler) Close() error {
	return h.Stop()
}

// ShouldWarm returns warm signals for server connections. Status frontends
// return nil so status pings never warm the target.
func (h *Handler) ShouldWarm() <-chan struct{} {
	return h.warm
}

func (h *Handler) accept(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				h.logger.Error("minecraft frontend accept failed", "err", err)
			}
			return
		}

		h.wg.Go(func() {
			h.handle(conn)
		})
	}
}

func (h *Handler) handle(conn net.Conn) {
	defer conn.Close()

	if h.warm != nil {
		select {
		case h.warm <- struct{}{}:
		default:
		}
	}

	_ = conn.SetDeadline(time.Now().Add(minecraftReadWindow))
	nextState, err := readHandshake(conn)
	if err != nil {
		h.logger.Debug("minecraft handshake failed", "remote", conn.RemoteAddr().String(), "err", err)
		return
	}

	switch nextState {
	case 1:
		if err := handleStatus(conn, h.status); err != nil {
			h.logger.Debug("minecraft status failed", "remote", conn.RemoteAddr().String(), "err", err)
		}
	case 2:
		if err := handleLogin(conn, h.message); err != nil {
			h.logger.Debug("minecraft login disconnect failed", "remote", conn.RemoteAddr().String(), "err", err)
		}
	default:
		h.logger.Debug("minecraft unsupported next state", "remote", conn.RemoteAddr().String(), "next_state", nextState)
	}
}

func handleStatus(conn net.Conn, status string) error {
	var pk mcpacket.Packet
	if err := pk.UnPack(conn, -1); err != nil {
		return err
	}
	if pk.ID != 0 {
		return fmt.Errorf("status request packet id %d", pk.ID)
	}
	response := mcpacket.Marshal(0, mcpacket.String(status))
	if err := response.Pack(conn, -1); err != nil {
		return err
	}

	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if err := pk.UnPack(conn, -1); err != nil {
		if isTimeout(err) || errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	if pk.ID != 1 || len(pk.Data) != 8 {
		return nil
	}
	return (&pk).Pack(conn, -1)
}

func handleLogin(conn net.Conn, message string) error {
	var pk mcpacket.Packet
	_ = pk.UnPack(conn, -1)
	response := mcpacket.Marshal(0, mcpacket.String(chatJSON(message)))
	return response.Pack(conn, -1)
}

func readHandshake(r io.Reader) (int, error) {
	var pk mcpacket.Packet
	if err := pk.UnPack(r, -1); err != nil {
		return 0, err
	}
	if pk.ID != 0 {
		return 0, fmt.Errorf("handshake packet id %d", pk.ID)
	}

	var protocol mcpacket.VarInt
	var address mcpacket.String
	var port mcpacket.UnsignedShort
	var nextState mcpacket.VarInt
	if err := pk.Scan(&protocol, &address, &port, &nextState); err != nil {
		return 0, err
	}
	return int(nextState), nil
}

func chatJSON(message string) string {
	data, _ := json.Marshal(map[string]string{"text": message})
	return string(data)
}

func statusJSON(text string) string {
	data, _ := json.Marshal(map[string]any{
		"version": map[string]any{
			"name":     "proxygw",
			"protocol": -1,
		},
		"players": map[string]any{
			"max":    0,
			"online": 0,
		},
		"description": map[string]string{
			"text": text,
		},
	})
	return string(data)
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func stringOption(options map[string]any, name, fallback string) (string, error) {
	value, ok := options[name]
	if !ok {
		return fallback, nil
	}
	result, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("minecraft frontend option %s must be a string", name)
	}
	return result, nil
}

func newHandler(name string, protocol config.Protocol, address netip.AddrPort, options map[string]any, warm bool) (frontend.Handler, error) {
	if protocol != config.ProtocolTCP {
		return nil, fmt.Errorf("minecraft frontend requires tcp protocol")
	}

	message, err := stringOption(options, "message", defaultMessage)
	if err != nil {
		return nil, err
	}
	statusText, err := stringOption(options, "status", defaultStatusText)
	if err != nil {
		return nil, err
	}

	var ch chan struct{}
	kind := "status"
	if warm {
		ch = make(chan struct{}, 1)
		kind = "server"
	}

	return &Handler{
		address: address,
		message: message,
		status:  statusJSON(statusText),
		warm:    ch,
		logger:  slog.Default().With("component", "minecraft:"+kind, "name", name),
	}, nil
}

// NewStatusHandler creates a Minecraft server-list status frontend. Status
// requests are answered locally and do not warm the target.
func NewStatusHandler(name string, protocol config.Protocol, address netip.AddrPort, options map[string]any) (frontend.Handler, error) {
	return newHandler(name, protocol, address, options, false)
}

// NewServerHandler creates a Minecraft login frontend. Connections are rejected
// with the configured message and warm the target.
func NewServerHandler(name string, protocol config.Protocol, address netip.AddrPort, options map[string]any) (frontend.Handler, error) {
	return newHandler(name, protocol, address, options, true)
}
