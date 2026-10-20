# csm-mail-paging-service

Polls Gmail, publishes one event per rule-matching email, and places the rule's
Twilio call path (`callPath`) when it has one.

- **Gmail label = idempotency.** A message is labelled processed only after all
  its events were published. Do not move that earlier, and do not add a second
  state store for correctness.
- **Headers only.** The client requests `format=metadata`; never read bodies.
- **Config is flat env strings** (`MAIL_TRIGGER_RULES` is one JSON string).
  Never commit credentials or real group addresses; tests use example.com.
- **`eventbus` and `poller.Envelope` mirror csm-notification-service** (separate
  Go modules, kept in sync by hand).
- Run `gosec -fmt=text ./...` and `govulncheck ./...` before a PR.
- **Twilio mirrors csm-notification-service** (`internal/twilio`, same `TWILIO_*`
  variables, `CALL_SENDING_ENABLED` disable-entirely convention). Credentials are
  required only when some rule has a `callPath`. Never log phone numbers; the
  message is escaped through `encoding/xml`, never concatenated into TwiML.
- **The call path is in-process and unacknowledged** (see the README's "Calls"
  limits). A durable, acknowledgement-aware ladder is csm-notification-service's
  `internal/paging`; do not grow a second one here without deciding to.
