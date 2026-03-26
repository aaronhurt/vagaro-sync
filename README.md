# vagaro-sync

`vagaro-sync` is a small macOS CLI that signs into Vagaro with a real browser, fetches upcoming appointments, and syncs them into Calendar.app.

## Background

I schedule my appointments a year in advance at my local barber, and they use Vagaro to manage their scheduling. I've
often wanted to be able to sync my appointments into my calendar without any manual effort.

I was also looking for ane excuse to test OpenAI's Codex tool for macOS. I've used similar tooling in the past but
wanted a real-world useful project to explore Codex.

I used Codex from the very beginning, just feeding it HAR exports from my browser, and a few curl commands to discover
the non-published Vagaro API. This was by far the biggest win so far in using Codex. The discovery and research saved
a lot of time. The initial code scaffolding and time to working prototype was faster than I could have done manually.

Now, for the not so good. I had to spend quite a bit of time getting Codex/GPT-5.4 to create code that I would be
comfortable maintaining without the use of an agentic assistant. Codex really wanted to provide test coverage for
_everything_, including stdlib functions and string checks against the generated javascript code. They were tests for
testing/coverage sake that provided little to no value and would have been hard to maintain.

In addition to test coverage, Codex also had a problem with code simplification. It loved to abstract things into functional
arguments and build-in "future flexibility" that would never be needed in this tool.

All-in-all, it was a good experience, and I came away with a working CLI that I can use to sync my appointments.

## Requirements

- macOS
- Go 1.26+
- Google Chrome or Chromium
- Calendar.app

Authentication is stored in the macOS keychain. Local sync state is stored at:

`~/Library/Application Support/vagaro-sync/state.json`

## Build

```bash
go build .
```

This produces the `vagaro-sync` binary in the repo root.

## Usage

Authenticate once:

```bash
./vagaro-sync auth-login
```

`auth-login` stores authentication only when the browser capture yields a well-formed, non-expired JWT.

Sync appointments into the `Vagaro Appointments` calendar:

```bash
./vagaro-sync sync
```

Clear stored authentication:

```bash
./vagaro-sync auth-clear
```

## Notes

- `sync` creates the `Vagaro Appointments` calendar if it does not exist.
- Re-running `sync` is incremental: unchanged appointments are skipped, changed appointments are updated, and missing calendar events are recreated.
- If the stored token is missing, malformed, or expired, re-run `./vagaro-sync auth-login`.
- Local-only integration tests are build-tagged and are not intended for CI:

```bash
go test -tags=integration ./...
```
