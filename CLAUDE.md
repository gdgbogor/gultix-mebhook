# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go project designed to handle webhook events from Pretix and route them to Firebase Cloud Messaging (FCM) using a service account. The project is intentionally kept simple.

## Development Commands

### Build and Run
```bash
go build -o pretix-webhook main.go
./pretix-webhook
```

### Development
```bash
go run main.go
```

### Dependencies
```bash
go mod tidy
go mod download
```

### Testing
```bash
go test ./...
go test -v ./...  # verbose output
go test -cover ./...  # with coverage
```

### Formatting and Linting
```bash
go fmt ./...
go vet ./...
```

## Architecture

- **Language**: Go 1.22.0
- **Purpose**: Webhook handler for Pretix events → FCM routing
- **Design Philosophy**: Simplicity first - minimal dependencies and straightforward implementation

## Project Structure

- `main.go` - Main application entry point
- `go.mod` - Go module definition
- `.serena/project.yml` - Serena AI assistant configuration

## Key Implementation Notes

- Routes webhook events from Pretix to Firebase Cloud Messaging
- Uses service account authentication for FCM via file path
- Handles webhook signature verification with HMAC-SHA256 (optional)
- Sends FCM notifications to a topic (configurable)
- Supports all Pretix order events (order.placed.require_approval, etc.)

## Environment Variables Required

Create a `.env` file with:
```bash
FCM_SERVICE_ACCOUNT_PATH=/path/to/firebase-service-account.json
FCM_PROJECT_ID=your-firebase-project-id
FCM_TOPIC=pretix-orders
PORT=8080
PRETIX_WEBHOOK_SECRET=your-webhook-secret  # Optional for security
```

## API Endpoints

- `POST /webhook` - Receives Pretix webhook events
- `GET /health` - Health check endpoint