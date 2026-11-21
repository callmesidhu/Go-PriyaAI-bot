package main

import (
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/ai"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/api"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/browser"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/config"
	"github.com/callmesidhu/Go-PriyaAI-bot/internal/database"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	c := config.Load()
	prompt, err := os.ReadFile("prompts/priya.txt")
	if err != nil {
		log.Fatal(err)
	}
	store, err := database.Open(c.DataPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if c.AdminToken == "" {
		log.Println("WARNING: PRIYA_ADMIN_TOKEN is empty; administrative API access is disabled")
	}
	server := api.New(store, ai.New(c.OllamaURL, c.Model, strings.TrimSpace(string(prompt))), &browser.Manager{}, c.AdminToken)
	log.Printf("PRIYA AI listening on http://%s", c.ListenAddr)
	log.Fatal(http.ListenAndServe(c.ListenAddr, server.Handler()))
}
