# Webhook Processor

A small Go HTTP service for receiving signed webhook events and retrieving them by ID. The service uses Gin and keeps events in memory; stored events are lost when the process stops and are not shared across instances.

This repository was created to test documentation tooling. The API reference is in [docs/api.md](docs/api.md).

## Run locally

Requires Go 1.22 or later. Set a shared secret and start the server:

```sh
export WEBHOOK_SECRET='replace-with-a-private-secret'
go run .
```

The server listens on port `8080` by default. Set `PORT` to use another port.
