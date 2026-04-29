package minecraft

import (
	"github.com/UselessMnemonic/proxygw/pkg/engine"
	"github.com/UselessMnemonic/proxygw/plugin"
	"github.com/UselessMnemonic/proxygw/plugins/minecraft/frontends"
)

func init() {
	err := plugin.Register("minecraft", plugin.Handler{
		OnLoad: func(_ map[string]any, _ *engine.Engine, namespace *plugin.Namespace) error {
			namespace.Frontends["server"] = frontends.NewServerHandler
			namespace.Frontends["status"] = frontends.NewStatusHandler
			return nil
		},
		OnUnload: func() error {
			return nil
		},
	})
	if err != nil {
		panic(err)
	}
}
