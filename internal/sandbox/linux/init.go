//go:build linux
// +build linux

package linux

import (
	"intentlens/internal/sandbox"
)

func init() {
	sandbox.RegisterLinuxSandbox(func(cfg sandbox.Config) (sandbox.Sandbox, error) {
		return NewLinuxSandbox(cfg)
	})
}
