package model

import "time"

// Channel provider types.
const (
	ChannelShoutrrr            = "shoutrrr"
	ChannelWhatsAppGreenAPI    = "whatsapp_greenapi"
	ChannelWhatsAppMultidevice = "whatsapp_multidevice"
)

// Channel is a notification destination. Its provider-specific config is stored
// AES-256-GCM encrypted and never returned in plaintext by the API.
type Channel struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Type             string    `json:"type"`
	ConfigEncrypted  []byte    `json:"-"`
	NeedsCredentials bool      `json:"needs_credentials"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
