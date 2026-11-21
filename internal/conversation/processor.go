package conversation

import (
	"context"
	"fmt"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/ai"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/browser"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/database"
	"strings"
	"sync"
)

type Processor struct {
	store *database.Store
	ai    *ai.Client
	mu    sync.Mutex
}

func New(store *database.Store, client *ai.Client) *Processor {
	return &Processor{store: store, ai: client}
}
func (p *Processor) Handle(ctx context.Context, in browser.IncomingMessage) (string, error) {
	if in.ID == "" || in.SenderID == "" || in.Text == "" {
		return "", fmt.Errorf("message id, sender, and text are required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	done, err := p.store.IsProcessed(ctx, in.ID)
	if err != nil {
		return "", err
	}
	if done {
		return "", nil
	}
	summary, memories, history, err := p.store.Context(ctx, in.SenderID)
	if err != nil {
		return "", err
	}
	turns := make([]ai.Turn, 0, len(history)+2)
	if summary != "" {
		turns = append(turns, ai.Turn{Role: "system", Content: "Conversation summary (use only when relevant; never invent details): " + summary})
	}
	if len(memories) > 0 {
		parts := make([]string, 0, len(memories))
		for _, m := range memories {
			parts = append(parts, m.Category+": "+m.Value)
		}
		turns = append(turns, ai.Turn{Role: "system", Content: "Known user preferences, only if useful: " + strings.Join(parts, "; ")})
	}
	for _, m := range history {
		role := "assistant"
		if m.Direction == "in" {
			role = "user"
		}
		turns = append(turns, ai.Turn{Role: role, Content: m.Content})
	}
	turns = append(turns, ai.Turn{Role: "user", Content: in.Text})
	reply, err := p.ai.GenerateTurns(ctx, turns)
	if err != nil {
		return "", err
	}
	ok, err := p.store.RecordExchange(ctx, in.ID, in.SenderID, in.Username, in.Text, reply)
	if err != nil || !ok {
		return "", err
	}
	return reply, nil
}
