package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"mailnest-be/internal/api"
	"mailnest-be/internal/config"
)

func main() {
	configPath := os.Getenv("MAILNEST_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	app, err := api.NewApp(cfg)
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("mailnest backend listening on %s", addr)
	if err := http.ListenAndServe(addr, app.Routes()); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
