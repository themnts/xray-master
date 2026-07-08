package provision

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	user string
}

func NewSSHClient(user string) *SSHClient {
	if user == "" {
		user = "root"
	}
	return &SSHClient{user: user}
}

func (c *SSHClient) Run(host, keyPath, script string) (string, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("read ssh key %s: %w", keyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(keyPEM)
	if err != nil {
		return "", fmt.Errorf("parse ssh key: %w", err)
	}

	cfg := &ssh.ClientConfig{
		User:            c.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	addr := net.JoinHostPort(host, "22")
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return "", fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf
	if err := session.Run(script); err != nil {
		out := bytes.TrimSpace(buf.Bytes())
		if len(out) > 0 {
			return "", fmt.Errorf("ssh run on %s: %w: %s", host, err, out)
		}
		return "", fmt.Errorf("ssh run on %s: %w", host, err)
	}
	return buf.String(), nil
}
