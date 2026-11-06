// +build darwin

package macos

import (
	"intentlens/internal/sandbox"
)

func init() {
	sandbox.RegisterMacOSSandbox(func(cfg sandbox.Config) (sandbox.Sandbox, error) {
		return NewMacOSSandbox(cfg)
	})
}
