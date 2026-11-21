package conversation

import (
	"context"
	"fmt"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/ai"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/browser"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/database"
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
	reply, err := p.ai.Generate(ctx, in.Text)
	if err != nil {
		return "", err
	}
	ok, err := p.store.RecordExchange(ctx, in.ID, in.SenderID, in.Username, in.Text, reply)
	if err != nil || !ok {
		return "", err
	}
	return reply, nil
}
