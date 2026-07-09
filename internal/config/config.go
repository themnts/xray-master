package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const DefaultPath = "/etc/xray-master/config.yaml"

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Provision    ProvisionConfig    `yaml:"provision"`
	Subscription SubscriptionConfig `yaml:"subscription"`
}

type ProvisionConfig struct {
	MasterIP       string `yaml:"master_ip"`
	EnrollTTLHours int    `yaml:"enroll_ttl_hours"`
	NodeAPIPort    int    `yaml:"node_api_port"`
}

type ServerConfig struct {
	Listen    string `yaml:"listen"`
	AdminKey  string `yaml:"admin_key"`
	PublicURL string `yaml:"public_url"`
	DBPath    string `yaml:"db_path"`
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

func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = "0.0.0.0:9480"
	}
	if cfg.Server.DBPath == "" {
		cfg.Server.DBPath = "/var/lib/xray-master/data.db"
	}
	if cfg.Subscription.UpdateIntervalHours <= 0 {
		cfg.Subscription.UpdateIntervalHours = 12
	}
	if cfg.Provision.NodeAPIPort <= 0 {
		cfg.Provision.NodeAPIPort = 9472
	}
	if cfg.Provision.EnrollTTLHours <= 0 {
		cfg.Provision.EnrollTTLHours = 24
	}
	return &cfg, nil
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
