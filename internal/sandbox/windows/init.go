// +build windows

package windows

import (
	"intentlens/internal/sandbox"
)

func init() {
	sandbox.RegisterWindowsSandbox(func(cfg sandbox.Config) (sandbox.Sandbox, error) {
		return NewWindowsSandbox(cfg)
	})
}
