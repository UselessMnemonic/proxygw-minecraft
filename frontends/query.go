package frontends

import (
	"errors"
	"net/netip"

	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/frontend"
)

// NewQueryHandler creates a Minecraft server-list status frontend. Status
// requests are answered locally and do not warm the target.
func NewQueryHandler(name string, protocol config.Protocol, address netip.AddrPort, options map[string]any) (frontend.Handler, error) {
	return nil, errors.New("not implemented")
}
