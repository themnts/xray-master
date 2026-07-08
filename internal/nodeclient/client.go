package nodeclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

type Inbound struct {
	ID             int       `json:"id"`
	Remark         string    `json:"remark"`
	Enable         bool      `json:"enable"`
	Port           int       `json:"port"`
	Protocol       string    `json:"protocol"`
	Settings       JSONField `json:"settings"`
	StreamSettings JSONField `json:"streamSettings"`
	Tag            string    `json:"tag"`
}

type JSONField struct {
	raw json.RawMessage
}

func (f *JSONField) UnmarshalJSON(data []byte) error {
	f.raw = append(f.raw[:0], data...)
	return nil
}

func (f JSONField) String() string {
	if len(f.raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(f.raw, &s) == nil {
		return s
	}
	return string(f.raw)
}

func (f JSONField) Map() (map[string]any, error) {
	if len(f.raw) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(f.raw, &m); err != nil {
		var s string
		if err2 := json.Unmarshal(f.raw, &s); err2 != nil {
			return nil, err
		}
		if err3 := json.Unmarshal([]byte(s), &m); err3 != nil {
			return nil, err3
		}
	}
	return m, nil
}

type ClientTraffic struct {
	Email string `json:"email"`
	Up    int64  `json:"up"`
	Down  int64  `json:"down"`
	Total int64  `json:"total"`
}

type AddClientRequest struct {
	InboundRemark string `json:"inbound_remark"`
	Email         string `json:"email"`
	UUID          string `json:"uuid"`
	Comment       string `json:"comment"`
	Enable        bool   `json:"-"`
}

type AddedClient struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Auth  string `json:"auth"`
}

func (c *Client) ListInbounds() ([]Inbound, error) {
	var out []Inbound
	if err := c.getJSON("/inbounds", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) AddClient(req AddClientRequest) (*AddedClient, error) {
	body := map[string]any{
		"inbound_remark": req.InboundRemark,
		"email":          req.Email,
		"uuid":           req.UUID,
	}
	if req.Comment != "" {
		body["comment"] = req.Comment
	}
	var out AddedClient
	if err := c.postJSON("/clients", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) SetClientEnabled(inboundRemark, email string, enabled bool) error {
	path := fmt.Sprintf("/clients/%s/%s?inbound=%s", url.PathEscape(email), action(enabled), url.QueryEscape(inboundRemark))
	return c.postJSON(path, nil, nil)
}

func (c *Client) ClientStats(inboundRemark, email string) (*ClientTraffic, error) {
	path := fmt.Sprintf("/clients/%s/stats?inbound=%s", url.PathEscape(email), url.QueryEscape(inboundRemark))
	var out ClientTraffic
	if err := c.getJSON(path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func action(enabled bool) string {
	if enabled {
		return "enable"
	}
	return "disable"
}

func (c *Client) getJSON(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) postJSON(path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var envelope struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &envelope)
		if envelope.Error != "" {
			return fmt.Errorf("node %s %s: %s", req.Method, req.URL.Path, envelope.Error)
		}
		return fmt.Errorf("node %s %s: HTTP %d", req.Method, req.URL.Path, resp.StatusCode)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}
