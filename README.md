# Webhook Processor

A small Go HTTP service for receiving signed webhook events and retrieving them by ID. Events are stored in memory, so they are lost when the server stops and are not shared across multiple server instances.

## Requirements

- Go 1.22 or later

## Start the server

Set `WEBHOOK_SECRET` to a private value that your webhook sender also knows, then run:

```sh
export WEBHOOK_SECRET='replace-with-a-private-secret'
go run .
```

The server listens on port `8080` by default. Set `PORT` to use another port.

## Event format

`POST /api/v1/webhooks` accepts this JSON envelope. `timestamp` must be an RFC3339 timestamp, and `data` can contain any valid JSON value.

```json
{
  "event_id": "evt_123",
  "event_type": "invoice.paid",
  "timestamp": "2026-01-02T03:04:05Z",
  "data": {
    "amount": 42,
    "currency": "USD"
  }
}
```

The JSON field names correspond to the Go `EventPayload` fields and tags: `EventID` (`json:"event_id"`), `EventType` (`json:"event_type"`), `Timestamp` (`json:"timestamp"`), and `Data` (`json:"data"`). An `ErrorResponse` uses `ErrorCode` (`json:"error_code"`) and `Message` (`json:"message"`), for example:

```json
{
  "error_code": "invalid_signature",
  "message": "A valid X-Signature header is required"
}
```

## Signature

Every POST must include `X-Signature: sha256=<hex>`, where `<hex>` is the lowercase hexadecimal HMAC-SHA256 of the exact raw request body using `WEBHOOK_SECRET` as the key. For example, calculate the signature for a saved `event.json` body with:

```sh
openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" event.json
```

Use the hexadecimal digest printed after `(stdin)=` or `event.json` as the value after `sha256=`. The server verifies the signature before parsing the JSON body.

## Happy path: event ingested

Save the event JSON shown above as `event.json`, then compute its signature:

```sh
SIGNATURE=$(openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" event.json | awk '{print $NF}')
curl -i http://localhost:8080/api/v1/webhooks \
  -X POST \
  -H 'Content-Type: application/json' \
  -H "X-Signature: sha256=$SIGNATURE" \
  --data-binary @event.json
```

A successful request returns `202 Accepted` and the stored event. Retrieve it with:

```sh
curl -i http://localhost:8080/api/v1/events/evt_123
```

## Sad path: invalid signature

```sh
curl -i http://localhost:8080/api/v1/webhooks \
  -X POST \
  -H 'Content-Type: application/json' \
  -H 'X-Signature: sha256=not-a-valid-signature' \
  --data-binary @event.json
```

This returns `401 Unauthorized` with a JSON `ErrorResponse`. A malformed JSON body with a valid signature returns `400 Bad Request`; looking up an event that was not ingested returns `404 Not Found`.
