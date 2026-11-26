//go:build darwin
// +build darwin

package macos

import (
	"intentlens/internal/noisolation"
)

func init() {
	noisolation.RegisterMacOSSandbox(func(cfg noisolation.Config) (noisolation.Sandbox, error) {
		return NewMacOSSandbox(cfg)
	})
}
