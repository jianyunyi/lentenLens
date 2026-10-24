package app

import (
	"intentlens/internal/explain"
	"testing"
)

func TestVerificationModeDefaultsAndPersistsInLocalSettings(t *testing.T) {
	a, _ := fixture(t)
	settings, err := a.Config()
	if err != nil || settings.Model.VerificationMode != explain.ModeLight {
		t.Fatalf("legacy/default config=%+v err=%v", settings.Model, err)
	}

	if err = a.Configure(Settings{Model: explain.Config{VerificationMode: explain.ModeStrict}}, false); err != nil {
		t.Fatal(err)
	}
	settings, err = a.Config()
	if err != nil || settings.Model.VerificationMode != explain.ModeStrict {
		t.Fatalf("persisted config=%+v err=%v", settings.Model, err)
	}
	if err = a.Configure(Settings{Model: explain.Config{VerificationMode: "unknown"}}, false); err == nil {
		t.Fatal("invalid verification mode accepted")
	}
}
