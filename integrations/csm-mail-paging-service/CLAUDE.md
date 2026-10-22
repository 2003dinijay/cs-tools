# csm-mail-paging-service

Polls Gmail and publishes one event per rule-matching email. No Twilio and no
paging logic lives here; consumers decide what to do with the event.

- **Gmail label = idempotency.** A message is labelled processed only after all
  its events were published. Do not move that earlier, and do not add a second
  state store for correctness.
- **Headers only.** The client requests `format=metadata`; never read bodies.
- **Config is flat env strings** (`MAIL_TRIGGER_RULES` is one JSON string).
  Never commit credentials or real group addresses; tests use example.com.
- **`eventbus` and `poller.Envelope` mirror csm-notification-service** (separate
  Go modules, kept in sync by hand).
- Run `gosec -fmt=text ./...` and `govulncheck ./...` before a PR.
