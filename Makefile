.PHONY: dev dev-server dev-web build run clean

STATIC = cmd/pmac-chat/static
PLACEHOLDER = <body>Follow the instructions in README.md to run a dev server or build a deployable server.</body>

# Load .env (if present) into the environment of the command that follows, the
# way ProfessorMAC runs `source .env && go run ./cmd/pmac`. set -a exports
# plain KEY=value lines too.
ENV = set -a; [ ! -f .env ] || . ./.env; set +a;

# Vite dev server with hot reload on http://localhost:5173 (proxies /api to
# the Go server on :8080), plus the Go server itself.
dev: front-end/node_modules
	$(MAKE) -j2 dev-server dev-web

dev-server:
	$(ENV) go run ./cmd/pmac-chat

dev-web: front-end/node_modules
	cd front-end && npm run dev

# Build the front end, copy it into the server's static directory (as
# ProfessorMAC's server.Dockerfile does), and build one binary with it embedded.
# The static directory is reset to its placeholder afterward.
build: front-end/node_modules
	cd front-end && npm run build
	cp -R front-end/dist/. $(STATIC)/
	go build -o pmac-chat ./cmd/pmac-chat
	rm -rf $(STATIC)/assets $(STATIC)/favicon.ico
	printf '%s\n' '$(PLACEHOLDER)' > $(STATIC)/index.html

# Build, then serve the app on http://localhost:8080.
run: build
	$(ENV) ./pmac-chat

front-end/node_modules: front-end/package.json
	cd front-end && npm install
	touch front-end/node_modules

clean:
	rm -rf front-end/dist pmac-chat
