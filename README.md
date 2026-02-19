# Design Changes and Improvements
While implementing this lab, I made several changes to improve the correctness, reliability, and clarity of the file transfer protocol. These changes were mainly focused on simplifying the protocol flow, improving error handling, and making the client and server behavior more predictable.

---
# Checksum Handling

Originally, the client sent the file data first and then sent a separate protobuf message containing the checksum. The server had to wait for this extra message after the file transfer finished. This made the protocol more fragile and harder to reason about.

I changed the design so that checksums are now sent as part of the metadata:

- During PUT, the client computes the checksum before sending any data and includes it in the StorageRequest.

- During GET, the server computes the checksum before sending the file and includes it in the RetrievalResponse.

This removes the need for an extra checksum message after streaming and makes the protocol easier to follow.

---
# Removal of Post Stream Messages

After this change, no protobuf messages are sent after file data is streamed. File transfers now consist of:

- A metadata message (request or response)

- A raw byte stream of the file

This reduces protocol complexity and avoids issues where the client and server could become out of sync.

---
# Improved PUT Behavior

The server now treats uploads as a single atomic operation:

- Disk space is checked before accepting a file.

- If an error occurs during transfer or the checksum does not match, the partially written file is deleted.

- The client receives a clear success or failure response at the end.

This prevents corrupted or incomplete files from being left on the server.

---
# Improved GET (Download) Behavior

The GET flow was also restructured to be safer:

- The server opens and checks the file before sending a successful response.

- The checksum is calculated before sending any file data.

- The client verifies the checksum after downloading the file.

- If verification fails, the downloaded file is deleted.

This ensures that the client never silently saves a corrupted file.

---
# Clearer Protocol Order

After these changes, the protocol follows a clear and consistent order.

### PUT

1. Client sends metadata (filename, size, checksum)

2. Server responds with OK or error

3. Client streams file bytes

4. Server sends final status

### GET

1. Client sends retrieval request

2. Server sends metadata (size, checksum)

3. Server streams file bytes

4. Client verifies checksum locally

This makes the behavior easier to debug and reason about.

---
# Security and Robustness

Additional improvements were made to make the system safer:

- filepath.Base() is used to prevent directory traversal.

- Temporary or invalid files are cleaned up immediately.

- Errors are handled explicitly instead of failing silently.

---
# MessageHandler Updates

The message handling layer was updated to support these changes:

Storage and retrieval messages now carry checksum metadata.

Retrieval responses return size and checksum together.

Client and server logic is kept separate from serialization logic.

---
# Compatibility Notes

To ensure compatibility with teammates:

- Everyone must use the same messages.proto file.

- Protobuf code must be regenerated after protocol changes.

- Clients and servers on different machines work as long as the protocol definitions match.

