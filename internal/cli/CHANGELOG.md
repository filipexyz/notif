# Changelog

All notable changes to the notif CLI will be documented in this file.

## [0.1.7] - 2026-01-03

### Added

- **emit**: `$input` variable in `--filter` expressions
  - Reference the emitted request data when filtering responses
  - Enables session-based filtering for multi-client scenarios
  - Example: `notif emit 'request' '{"session_id":"abc"}' --reply-to 'response' --filter '.session_id == $input.session_id'`

## [0.1.6] - 2026-01-03

### Added

- **emit**: `--raw` flag to output only the response data field
  - Useful for hooks and pipes that need just the JSON payload
  - Example: `notif emit 'request' '{}' --reply-to 'response' --raw`

## [0.1.5] - 2026-01-03

### Added

- **emit**: Request-response mode with `--reply-to`, `--filter`, and `--timeout` flags
  - Emit an event and wait for a matching response on specified topics
  - Use jq expressions to filter responses
  - Example: `notif emit 'tasks.create' '{"id":1}' --reply-to 'tasks.done' --filter '.id == 1' --timeout 30s`

- **subscribe**: Filter and auto-exit with `--filter`, `--once`, `--count`, and `--timeout` flags
  - Filter events using jq expressions
  - Exit after first match (`--once`) or N matches (`--count`)
  - Timeout if no matching events received
  - Example: `notif subscribe 'orders.*' --filter '.amount > 100' --once --timeout 60s`

## [0.1.4] - Previous

- Initial CLI release with `emit`, `subscribe`, `auth`, and webhook commands
