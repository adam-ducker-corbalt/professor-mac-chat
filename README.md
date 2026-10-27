# ProfessorMAC Chat

A prototype for asking ProfessorMAC questions in natural language. A chat
launcher in the corner of a ProfessorMAC-styled page sends questions to the Go
server, which answers them with OpenAI, calling tools that read sample DORA
metrics and cost findings.

## Layout

The layout follows [mac-fc-professor-mac](https://github.com/Enterprise-CMCS/mac-fc-professor-mac)
so the code can move into it later.

Directory | Description | ProfessorMAC equivalent
--- | --- | ---
`front-end/` | React + TypeScript app (Vite), using the CMS Design System | `front-end/`
`cmd/pmac-chat/` | Server: embeds the built front end and serves it with the chat API | `cmd/pmac/`
`internal/app/chat/` | Chat API: the OpenAI agent loop, the tools it can call, and the sample data they read | a feature package beside `internal/app/devopscenter/`
`internal/envconfig/` | Reads configuration from environment variables | `internal/envconfig/`

As in ProfessorMAC, the built front end is copied into the server's `static/`
directory at build time (only a placeholder `index.html` is committed), assets
are served under `/static/`, and every other path returns `index.html`.

## Prerequisites

- Go (version in `go.mod`)
- Node.js and npm

## Configuration

Copy the sample config and add your OpenAI API key:

```bash
cp .env.example .env
```

`.env` is git-ignored. `make dev` and `make run` load it automatically. To run
the server by hand, source it first as in ProfessorMAC:

```bash
source .env && go run ./cmd/pmac-chat
```

## Running

```bash
make dev    # Vite with hot reload on http://localhost:5173, plus the Go server on :8080
make run    # build one binary (./pmac-chat) with the front end embedded and serve it on :8080
go test ./...
```
