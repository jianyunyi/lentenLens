//go:build windows
// +build windows

package windows

import (
	"intentlens/internal/noisolation"
)

func init() {
	noisolation.RegisterWindowsSandbox(func(cfg noisolation.Config) (noisolation.Sandbox, error) {
		return NewWindowsSandbox(cfg)
	})
}
