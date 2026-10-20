# csm-mail-paging-service

Polls a Gmail mailbox and publishes an event to the event bus for every email
that matches a configured rule. It does not place calls: a consumer (for
example csm-notification-service's paging engine) reacts to the event.

```
Gmail (group mail) -> poll -> rules -> event bus (Envelope) -> consumer
```

## How it works

1. Every `MAIL_POLL_INTERVAL` it lists messages in the delegated mailbox that
   were sent to a rule's `recipient` (To, Cc or Delivered-To), are newer than
   `MAIL_LOOKBACK` and do not carry the processed label.
2. It reads only the headers (From, To, Cc, Subject, Message-ID), never the body.
3. For each matching rule it publishes `{type: eventType, entityId: <Message-ID>, payload}`.
4. Only after every publish succeeded does it add the processed label. A failed
   publish leaves the message unlabelled, so the next pass retries it
   (at-least-once). Consumers dedupe on the payload's `dedupeKey`
   (`<Message-ID>/<rule name>`), not on the Message-ID alone: one email that
   matches two rules produces two events.

   Retries are bounded by `MAIL_LOOKBACK` (default `2d`): a message still
   unpublished after that window is no longer listed. Alert on the
   "mail message failed; will retry" log line.

The Gmail label is the durable record of "handled", so no database or Redis is
needed for correctness.

## Rules (`MAIL_TRIGGER_RULES`)

| Field | Description |
|-------|-------------|
| `name` | Unique rule name, carried in the payload |
| `recipient` | Group or mailbox address the email was sent to |
| `senderRegex` | Optional: only emails from matching senders |
| `eventType` | Event type published on a match |
| `callPath` | Optional ordered phone path: `[{"level":2,"to":["+E164",...]},{"level":3,...}]`. Numbers of one level are called together, then the next level after `levelDelaySeconds`. Omit to publish the event only |
| `levelDelaySeconds` | Wait between levels |
| `callMessage` | Spoken before the email subject (default "Email alert received") |
| `levels` | Passed to the consumer untouched; this service does not use it (see below) |

A new source is another entry in the array.

## Calls (Twilio)

When a rule matches and has a `callPath`, the service places the calls itself
through Twilio, configured with the same `TWILIO_*` variables as
csm-notification-service (see `.env.example`); Twilio credentials are required
only if a rule has a `callPath`. `CALL_SENDING_ENABLED=false` turns calling off
while still publishing events. The call reads `callMessage` plus the email
subject (cut to 200 characters; escaped, so it cannot inject TwiML).

Limits to know: the path runs in this process, so a restart mid-path drops the
levels not yet reached; there is no acknowledgement, so every level is called
after its delay regardless of whether an earlier level answered; and a retried
message (publish failure) can repeat a level that already rang. Call numbers
are never logged.

## Google setup

A service account with domain-wide delegation for the `gmail.modify` scope, and
a delegated user that is a member of the group. See `.env.example`.

## Commands

```bash
go test -race ./...
go run ./cmd/server
```
