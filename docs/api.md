# Webhook Processor API

The API listens on the server's configured port (`8080` by default). Set `WEBHOOK_SECRET` to the secret shared with the webhook sender before starting the service.

## `POST /api/v1/webhooks`

Accepts and stores a signed webhook event. Send a JSON request body with all of these fields:

| Field | Type | Description |
| --- | --- | --- |
| `event_id` | string | Unique identifier used to retrieve the event. Must not be empty or whitespace. |
| `event_type` | string | Event name, such as `invoice.paid`. Must not be empty or whitespace. |
| `timestamp` | string | Event time in RFC3339 format. |
| `data` | JSON value | Event-specific data. Any valid JSON value is accepted. |

Example:

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

### Request signature

Every request must include an `X-Signature` header in this format:

```text
X-Signature: sha256=<hex-digest>
```

`<hex-digest>` is the lowercase hexadecimal HMAC-SHA256 of the exact raw request body, using the value of `WEBHOOK_SECRET` as the key. The signature is verified before the body is parsed. For example, compute the digest for a saved `event.json` file with:

```sh
openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" event.json
```

### Responses

- `202 Accepted`: the event was stored; the response body contains the event payload.
- `400 Bad Request`: the body could not be read, is malformed or invalid JSON, does not contain a valid RFC3339 timestamp, or is missing a required field. The `error_code` is `malformed_json` for body-read/JSON/timestamp errors or `invalid_event` for missing fields.
- `401 Unauthorized`: the signature is missing, malformed, or does not match. The `error_code` is `invalid_signature`.

Error responses have this shape:

```json
{
  "error_code": "invalid_signature",
  "message": "A valid X-Signature header is required"
}
```

## `GET /api/v1/events/:event_id`

Returns the stored event whose `event_id` matches the path parameter. For example:

```sh
curl -i http://localhost:8080/api/v1/events/evt_123
```

### Responses

- `200 OK`: the response body contains the event payload.
- `404 Not Found`: no event was found for the requested ID. The `error_code` is `event_not_found`.

Events are held in memory, so they are available only to the running process that received them and are lost when it stops.
