//go:build linux
// +build linux

package linux

import (
	"intentlens/internal/noisolation"
)

func init() {
	noisolation.RegisterLinuxSandbox(func(cfg noisolation.Config) (noisolation.Sandbox, error) {
		return NewLinuxSandbox(cfg)
	})
}
