package provision

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/thethoughtcriminal/xray-master/internal/config"
	"github.com/thethoughtcriminal/xray-master/internal/nodeclient"
)

type Result struct {
	APIKey  string
	APIURL  string
	APIPort int
}

type Provisioner struct {
	cfg config.ProvisionConfig
	ssh *SSHClient
}

func New(cfg config.ProvisionConfig) *Provisioner {
	return &Provisioner{
		cfg: cfg,
		ssh: NewSSHClient(cfg.SSHUser),
	}
}

func (p *Provisioner) Provision(ip string) (*Result, error) {
	if ip = strings.TrimSpace(ip); ip == "" {
		return nil, fmt.Errorf("ip is required")
	}
	if p.cfg.SSHKeyPath == "" {
		return nil, fmt.Errorf("provision.ssh_key_path is required")
	}

	masterIP, err := p.masterIP()
	if err != nil {
		return nil, err
	}
	port := p.cfg.NodeAPIPort
	if port <= 0 {
		port = 9472
	}
	installURL := p.cfg.XRayNodeInstallScript
	if installURL == "" {
		installURL = "https://raw.githubusercontent.com/thethoughtcriminal/xray-node/main/scripts/install.sh"
	}

	script := fmt.Sprintf(`set -euo pipefail
MASTER_IP=%q
NODE_API_PORT=%d
INSTALL_URL=%q
CONFIG=/etc/xray-node/config.yaml

if ! command -v xray-node >/dev/null 2>&1; then
  echo "Installing xray-node..."
  curl -fsSL "$INSTALL_URL" | bash
fi

if [[ ! -f "$CONFIG" ]]; then
  echo "xray-node config not found at $CONFIG" >&2
  exit 1
fi

sed -i "s/^[[:space:]]*listen:.*/  listen: 0.0.0.0:${NODE_API_PORT}/" "$CONFIG"
systemctl restart xray-node

if command -v ufw >/dev/null 2>&1; then
  ufw allow from "$MASTER_IP" to any port "$NODE_API_PORT" >/dev/null 2>&1 || true
fi

API_KEY="$(awk '/^  key:/ { print $2; exit }' "$CONFIG")"
if [[ -z "$API_KEY" ]]; then
  echo "could not read api key from $CONFIG" >&2
  exit 1
fi

echo "XRAY_NODE_API_KEY=$API_KEY"
`, masterIP, port, installURL)

	out, err := p.ssh.Run(ip, p.cfg.SSHKeyPath, script)
	if err != nil {
		return nil, err
	}

	apiKey := parseAPIKey(out)
	if apiKey == "" {
		return nil, fmt.Errorf("provision on %s: api key not found in script output", ip)
	}

	apiURL := fmt.Sprintf("http://%s", net.JoinHostPort(ip, fmt.Sprintf("%d", port)))
	client := nodeclient.New(apiURL, apiKey)
	deadline := time.Now().Add(2 * time.Minute)
	for {
		if _, err := client.ListInbounds(); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("node %s API not reachable at %s after install", ip, apiURL)
		}
		time.Sleep(3 * time.Second)
	}

	return &Result{
		APIKey:  apiKey,
		APIURL:  apiURL,
		APIPort: port,
	}, nil
}

func (p *Provisioner) masterIP() (string, error) {
	if ip := strings.TrimSpace(p.cfg.MasterIP); ip != "" {
		return ip, nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return "", fmt.Errorf("detect master public ip: %w (set provision.master_ip in config)", err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	ip := strings.TrimSpace(string(buf[:n]))
	if ip == "" {
		return "", fmt.Errorf("detect master public ip: empty response (set provision.master_ip in config)")
	}
	return ip, nil
}

func parseAPIKey(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "XRAY_NODE_API_KEY=") {
			return strings.TrimPrefix(line, "XRAY_NODE_API_KEY=")
		}
	}
	return ""
}
