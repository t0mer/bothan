package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/containrrr/shoutrrr"

	"github.com/t0mer/bothan/internal/model"
)

// Provider config shapes (stored as JSON, encrypted at rest).

// ShoutrrrConfig is a single Shoutrrr URL (Telegram, Slack, Discord, SMTP, …).
type ShoutrrrConfig struct {
	URL string `json:"url"`
}

// GreenAPIConfig is a GreenAPI WhatsApp cloud channel.
type GreenAPIConfig struct {
	InstanceID string `json:"instance_id"`
	Token      string `json:"token"`
	Phone      string `json:"phone"`
	APIURL     string `json:"api_url"`
}

// WhatsAppMultideviceConfig targets a self-hosted go-whatsapp-web-multidevice.
type WhatsAppMultideviceConfig struct {
	BaseURL  string `json:"base_url"`
	Phone    string `json:"phone"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Dispatcher sends messages through a channel provider.
type Dispatcher struct {
	http *http.Client
}

// NewDispatcher builds a Dispatcher with the given HTTP client (or a default).
func NewDispatcher(hc *http.Client) *Dispatcher {
	if hc == nil {
		hc = &http.Client{}
	}
	return &Dispatcher{http: hc}
}

// Send dispatches a message to a channel given its plaintext config JSON.
func (d *Dispatcher) Send(ctx context.Context, channelType string, configJSON []byte, message string) error {
	switch channelType {
	case model.ChannelShoutrrr:
		var c ShoutrrrConfig
		if err := json.Unmarshal(configJSON, &c); err != nil {
			return fmt.Errorf("invalid shoutrrr config: %w", err)
		}
		return d.sendShoutrrr(c, message)
	case model.ChannelWhatsAppGreenAPI:
		var c GreenAPIConfig
		if err := json.Unmarshal(configJSON, &c); err != nil {
			return fmt.Errorf("invalid greenapi config: %w", err)
		}
		return d.sendGreenAPI(ctx, c, message)
	case model.ChannelWhatsAppMultidevice:
		var c WhatsAppMultideviceConfig
		if err := json.Unmarshal(configJSON, &c); err != nil {
			return fmt.Errorf("invalid whatsapp config: %w", err)
		}
		return d.sendMultidevice(ctx, c, message)
	default:
		return fmt.Errorf("unknown channel type %q", channelType)
	}
}

func (d *Dispatcher) sendShoutrrr(c ShoutrrrConfig, message string) error {
	url := strings.TrimSpace(c.URL)
	if url == "" {
		return fmt.Errorf("shoutrrr url is required")
	}
	if err := shoutrrr.Send(url, message); err != nil {
		return fmt.Errorf("shoutrrr send: %w", err)
	}
	return nil
}

func (d *Dispatcher) sendGreenAPI(ctx context.Context, c GreenAPIConfig, message string) error {
	instance := strings.TrimSpace(c.InstanceID)
	token := strings.TrimSpace(c.Token)
	phone := strings.TrimSpace(c.Phone)
	apiURL := strings.TrimRight(strings.TrimSpace(c.APIURL), "/")
	if apiURL == "" {
		apiURL = "https://api.green-api.com"
	}
	if instance == "" || token == "" || phone == "" {
		return fmt.Errorf("greenapi requires instance_id, token, and phone")
	}
	chatID := phone
	if !strings.Contains(chatID, "@") {
		chatID += "@c.us"
	}
	body, _ := json.Marshal(map[string]string{"chatId": chatID, "message": message})
	url := fmt.Sprintf("%s/waInstance%s/sendMessage/%s", apiURL, instance, token)
	return d.postJSON(ctx, url, body, "", "")
}

func (d *Dispatcher) sendMultidevice(ctx context.Context, c WhatsAppMultideviceConfig, message string) error {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	phone := strings.TrimSpace(c.Phone)
	if base == "" || phone == "" {
		return fmt.Errorf("whatsapp multidevice requires base_url and phone")
	}
	body, _ := json.Marshal(map[string]string{"phone": phone, "message": message})
	return d.postJSON(ctx, base+"/send/message", body, c.Username, c.Password)
}

func (d *Dispatcher) postJSON(ctx context.Context, url string, body []byte, user, pass string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := d.http.Do(req)
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("provider returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}
