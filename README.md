# vagaro-sync

`vagaro-sync` is a small macOS CLI that signs into Vagaro with a real browser,
fetches upcoming appointments, and syncs them into Calendar.app.

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

Sync appointments into the `Vagaro Appointments` calendar:

```bash
./vagaro-sync sync
```

Clear stored authentication:

```bash
./vagaro-sync auth-clear
```

Print version information:

```bash
./vagaro-sync version
```

## Notes

- `sync` creates the `Vagaro Appointments` calendar if it does not exist.
- Re-running `sync` is incremental and will recreate missing calendar events.
