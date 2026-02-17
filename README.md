# file-transfer

A TCP file transfer client and server built with Go and Protocol Buffers.

## Usage

### Server
```bash
./bin/server <port> [download-dir]
./bin/server 9898 ./storage/
```

### Client
```bash
./bin/client <host:port> <put|get> <file-name> [destination-dir]
./bin/client localhost:9898 put /path/to/file.jpg
./bin/client localhost:9898 get file.jpg /tmp/downloads/
```

## Building

```bash
make
```

To regenerate protobuf code:
```bash
cd proto && bash build.sh
```

## Bug Fixes

1. **Server didn't respond after PUT checksum** — Client hung forever waiting for a response after file upload. Added `SendResponse` after checksum verification.

2. **Server crashed on GET for nonexistent file** — `log.Fatalln` killed the entire server process. Replaced with an error response so the server stays alive.

3. **Client sent full file path as filename** — PUT with `/tmp/file.txt` stored the file at that absolute path instead of in the server's download directory. Now sends only the basename.

4. **GET destination directory ignored** — The optional destination directory argument was parsed but never passed to the `get()` function. Now uses `filepath.Join(dir, fileName)` to save files in the correct location.

5. **No disk space check before PUT** — Server accepted files without verifying available disk space. Added `syscall.Statfs` check before accepting a transfer.

6. **Error in receive loop didn't terminate** — When `Receive()` returned an error, `handleClient` logged it but kept looping instead of returning. Now exits cleanly on error.

7. **Send/Receive ignored ReadN/WriteN errors** — Errors from `ReadN`/`WriteN` in `message_handler.go` were silently swallowed. Now properly propagated.

8. **Dead code after log.Fatalln** — Removed unreachable `os.Exit(1)` after `log.Fatalln` (which already calls `os.Exit`).

## Cross-Compatibility Changes

Adopted proto changes from the `paria` branch to align with teammate's protocol:

- **`StorageRequest`** — Added `bytes checksum = 3`. The client now pre-computes the MD5 checksum and sends it in the initial PUT request, instead of sending it as a separate `ChecksumVerification` message after the file transfer.

- **`RetrievalResponse`** — Added `bytes checksum = 3`. The server now pre-computes the MD5 checksum and includes it in the GET response, instead of sending it as a separate message after streaming the file.

Updated `go.mod` from Go 1.19 to Go 1.21 to support the regenerated protobuf code.
