package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCLISettings(t *testing.T) {
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	groveDir := filepath.Join(tmpDir, "my-grove")
	groveScionDir := filepath.Join(groveDir, ".scion")
	if err := os.MkdirAll(groveScionDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Test defaults (embedded)
	s, err := LoadSettings(groveScionDir)
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}
	if s.CLI == nil {
		t.Fatal("expected CLI settings to be non-nil")
	}
	if s.CLI.AutoHelp == nil {
		t.Fatal("expected CLI.AutoHelp to be non-nil")
	}
	if *s.CLI.AutoHelp != true {
		t.Errorf("expected default autohelp true, got %v", *s.CLI.AutoHelp)
	}

	// 2. Test override via UpdateSetting
	err = UpdateSetting(groveScionDir, "cli.autohelp", "false", false)
	if err != nil {
		t.Fatalf("UpdateSetting failed: %v", err)
	}

	s, err = LoadSettings(groveScionDir)
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}
	if s.CLI == nil || s.CLI.AutoHelp == nil || *s.CLI.AutoHelp != false {
		t.Errorf("expected autohelp false after update, got %v", s.CLI.AutoHelp)
	}

	// 3. Test GetSettingValue
	val, err := GetSettingValue(s, "cli.autohelp")
	if err != nil {
		t.Fatalf("GetSettingValue failed: %v", err)
	}
	if val != "false" {
		t.Errorf("expected GetSettingValue 'false', got '%s'", val)
	}

	// 4. Test GetSettingsMap
	m := GetSettingsMap(s)
	if m["cli.autohelp"] != "false" {
		t.Errorf("expected GetSettingsMap to have cli.autohelp=false, got %s", m["cli.autohelp"])
	}
}
