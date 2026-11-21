package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
)

type Client struct {
	URL, Model, Prompt string
	HTTP               *http.Client
}
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Stream   bool      `json:"stream"`
	Options  options   `json:"options"`
}
type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type options struct {
	NumCtx      int     `json:"num_ctx"`
	Temperature float64 `json:"temperature"`
	NumPredict  int     `json:"num_predict"`
}
type chatResponse struct {
	Message message `json:"message"`
}
type Turn struct{ Role, Content string }

func (c *Client) Healthy(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.URL, "/")+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama health check: %s", resp.Status)
	}
	return nil
}
func (c *Client) Generate(ctx context.Context, input string) (string, error) {
	return c.GenerateTurns(ctx, []Turn{{Role: "user", Content: input}})
}
func (c *Client) GenerateTurns(ctx context.Context, turns []Turn) (string, error) {
	messages := []message{{Role: "system", Content: c.Prompt}}
	for _, turn := range turns {
		messages = append(messages, message{Role: turn.Role, Content: turn.Content})
	}
	payload, _ := json.Marshal(chatRequest{Model: c.Model, Messages: messages, Options: options{2048, .8, 100}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.URL, "/")+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("ollama: %s: %s", resp.Status, b)
	}
	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return Validate(out.Message.Content)
}
func Validate(reply string) (string, error) {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return "", fmt.Errorf("empty model response")
	}
	if len([]rune(reply)) > 600 {
		return "", fmt.Errorf("response too long")
	}
	for _, r := range reply {
		if unicode.In(r, unicode.Malayalam) {
			return "", fmt.Errorf("response contains Malayalam Unicode")
		}
	}
	return reply, nil
}
func New(url, model, prompt string) *Client {
	return &Client{url, model, prompt, &http.Client{Timeout: 45 * time.Second}}
}
