package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// payload is the shape Discord expects.
type payload struct {
	Content string `json:"content"`
}

// Send posts a message to the Discord channel.
// It does nothing if DISCORD_WEBHOOK_URL is not set.
func Send(message string) {
	url := os.Getenv("DISCORD_WEBHOOK_URL")
	if url == "" {
		return
	}

	body, err := json.Marshal(payload{Content: message})
	if err != nil {
		log.Printf("notify: building payload: %v", err)
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("notify: posting to discord: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("notify: discord returned %d", resp.StatusCode)
	}
}

// Drugf is a small helper so callers can format a message inline.
func Drugf(format string, args ...any) {
	Send(fmt.Sprintf(format, args...))
}
