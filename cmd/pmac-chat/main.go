// Command pmac-chat serves the ProfessorMAC Chat front end and its chat API.
//
// Like cmd/pmac in ProfessorMAC, the built front end (front-end/dist) is
// copied into ./static at build time and embedded in the binary. Assets are
// served under /static/ and every other path gets index.html so client-side
// routes work. Configuration comes from environment variables (see
// .env.example); `make` sources .env before running.
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"professor-mac-chat/internal/app/chat"
	"professor-mac-chat/internal/envconfig"
)

//go:embed static/*
var staticFS embed.FS

var config = struct {
	port            string
	openAIKey       string
	openAIModel     string
	openAILogBodies bool
	weatherAgent    string
}{}

func loadConfigFromEnv() error {
	config.port = envconfig.StringDefault("PROFESSOR_MAC_API_PORT", "8080")
	config.openAIKey = envconfig.String("OPENAI_API_KEY")
	config.openAIModel = envconfig.StringDefault("OPENAI_MODEL", "gpt-5-mini")
	logBodies, err := envconfig.BoolDefault("OPENAI_LOG_BODIES", false)
	if err != nil {
		return err
	}
	config.openAILogBodies = logBodies
	config.weatherAgent = envconfig.StringDefault("WEATHER_USER_AGENT", chat.WeatherUserAgent)
	return nil
}

func main() {
	if err := loadConfigFromEnv(); err != nil {
		log.Fatal(err)
	}
	chat.WeatherUserAgent = config.weatherAgent

	// Never log the key itself.
	var client *chat.Client
	if config.openAIKey == "" || config.openAIKey == "your-openai-api-key-here" {
		log.Print("OPENAI_API_KEY is not set; copy .env.example to .env and add your key")
	} else {
		client = chat.NewClient(config.openAIKey, config.openAIModel, config.openAILogBodies)
		log.Printf("Chat uses OpenAI model %s (full request/response logging: %t)", config.openAIModel, config.openAILogBodies)
	}

	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(static))

	mux := http.NewServeMux()
	chat.Register(mux, client)
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static, "favicon.ico")
	})
	// All other requests are routes in the React app, which index.html serves.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static, "index.html")
	})

	log.Printf("ProfessorMAC Chat on http://localhost:%s", config.port)
	log.Fatal(http.ListenAndServe(":"+config.port, mux))
}
