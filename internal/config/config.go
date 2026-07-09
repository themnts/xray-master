package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultPath             = "/etc/xray-master/config.yaml"
	DefaultSubscriptionPath = "/etc/xray-master/subscription.yaml"
)

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Enroll       EnrollConfig       `yaml:"enroll"`
	Subscription SubscriptionConfig `yaml:"-"`
}

type EnrollConfig struct {
	MasterIP       string `yaml:"master_ip"`
	EnrollTTLHours int    `yaml:"enroll_ttl_hours"`
}

type ServerConfig struct {
	Listen           string `yaml:"listen"`
	AdminKey         string `yaml:"admin_key"`
	PublicURL        string `yaml:"public_url"`
	DBPath           string `yaml:"db_path"`
	SubscriptionPath string `yaml:"subscription_path"`
}

type SubscriptionConfig struct {
	UpdateIntervalHours int             `yaml:"update_interval_hours"`
	Profiles            []ProfileConfig `yaml:"profiles"`
}

// ProfileConfig defines how nodes appear in a user's subscription.
type ProfileConfig struct {
	Name    string         `yaml:"name"`
	Mode    string         `yaml:"mode"` // smart_multi | single
	Entries []ProfileEntry `yaml:"entries"`
}

type ProfileEntry struct {
	Node    string `yaml:"node"`
	Inbound string `yaml:"inbound"`
	Label   string `yaml:"label"`
}

type configFile struct {
	Server       ServerConfig       `yaml:"server"`
	Enroll       EnrollConfig       `yaml:"enroll"`
	Provision    EnrollConfig       `yaml:"provision"` // deprecated alias for enroll
	Subscription SubscriptionConfig `yaml:"subscription"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var file configFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg := Config{
		Server:       file.Server,
		Enroll:       file.Enroll,
		Subscription: file.Subscription,
	}
	mergeLegacyProvision(&cfg, file.Provision)
	applyDefaults(&cfg, path)

	if len(cfg.Subscription.Profiles) > 0 {
		applySubscriptionDefaults(&cfg.Subscription)
	} else {
		sub, err := loadSubscriptionFile(cfg.Server.SubscriptionPath)
		if err != nil {
			return nil, err
		}
		cfg.Subscription = *sub
		applySubscriptionDefaults(&cfg.Subscription)
	}
	return &cfg, nil
}

func loadSubscriptionFile(path string) (*SubscriptionConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read subscription %s: %w", path, err)
	}
	var sub SubscriptionConfig
	if err := yaml.Unmarshal(data, &sub); err != nil {
		return nil, fmt.Errorf("parse subscription %s: %w", path, err)
	}
	return &sub, nil
}

func mergeLegacyProvision(cfg *Config, legacy EnrollConfig) {
	if cfg.Enroll.MasterIP == "" {
		cfg.Enroll.MasterIP = legacy.MasterIP
	}
	if cfg.Enroll.EnrollTTLHours <= 0 && legacy.EnrollTTLHours > 0 {
		cfg.Enroll.EnrollTTLHours = legacy.EnrollTTLHours
	}
}

func applyDefaults(cfg *Config, mainPath string) {
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = "0.0.0.0:9480"
	}
	if cfg.Server.DBPath == "" {
		cfg.Server.DBPath = "/var/lib/xray-master/data.db"
	}
	if cfg.Server.SubscriptionPath == "" {
		if mainPath == DefaultPath {
			cfg.Server.SubscriptionPath = DefaultSubscriptionPath
		} else {
			cfg.Server.SubscriptionPath = filepath.Join(filepath.Dir(mainPath), "subscription.yaml")
		}
	}
	if cfg.Enroll.EnrollTTLHours <= 0 {
		cfg.Enroll.EnrollTTLHours = 24
	}
}

func applySubscriptionDefaults(sub *SubscriptionConfig) {
	if sub.UpdateIntervalHours <= 0 {
		sub.UpdateIntervalHours = 12
	}
}

func (c *Config) Validate() error {
	if c.Server.AdminKey == "" {
		return fmt.Errorf("server.admin_key is required")
	}
	if c.Server.PublicURL == "" {
		return fmt.Errorf("server.public_url is required (used in subscription links)")
	}
	for i, p := range c.Subscription.Profiles {
		if p.Name == "" {
			return fmt.Errorf("subscription.profiles[%d].name is required", i)
		}
		if p.Mode != "smart_multi" && p.Mode != "single" {
			return fmt.Errorf("subscription.profiles[%d].mode must be smart_multi or single", i)
		}
		if len(p.Entries) == 0 {
			return fmt.Errorf("subscription.profiles[%d] must have at least one entry", i)
		}
		for j, e := range p.Entries {
			if e.Node == "" || e.Inbound == "" {
				return fmt.Errorf("subscription.profiles[%d].entries[%d]: node and inbound are required", i, j)
			}
		}
	}
	return nil
}
