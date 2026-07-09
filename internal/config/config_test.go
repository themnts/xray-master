package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSplitSubscription(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "config.yaml")
	subPath := filepath.Join(dir, "subscription.yaml")

	if err := os.WriteFile(mainPath, []byte(`
server:
  admin_key: test-key
  public_url: https://sub.example.com
  subscription_path: `+subPath+`
enroll:
  master_ip: "203.0.113.1"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(subPath, []byte(`
profiles:
- name: p1
  mode: single
  entries:
  - node: nl-1
    inbound: vless-reality
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Subscription.Profiles) != 1 {
		t.Fatalf("profiles: got %d", len(cfg.Subscription.Profiles))
	}
	if cfg.Subscription.Profiles[0].Name != "p1" {
		t.Fatalf("profile name: %q", cfg.Subscription.Profiles[0].Name)
	}
	if cfg.Subscription.UpdateIntervalHours != 12 {
		t.Fatalf("update interval: %d", cfg.Subscription.UpdateIntervalHours)
	}
	if cfg.Enroll.MasterIP != "203.0.113.1" {
		t.Fatalf("master_ip: %q", cfg.Enroll.MasterIP)
	}
}

func TestLoadRequiresSubscriptionFile(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(`
server:
  admin_key: test-key
  public_url: https://sub.example.com
  subscription_path: /nonexistent/subscription.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(mainPath); err == nil {
		t.Fatal("expected error for missing subscription file")
	}
}
