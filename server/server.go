package main

import (
	"crypto/md5"
	"file-transfer/messages"
	"file-transfer/util"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

func ensureEnoughDiskSpace(path string, needed uint64) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return err
	}

	available := stat.Bavail * uint64(stat.Bsize)
	if available < needed {
		return fmt.Errorf("not enough disk space: need %d bytes, available %d bytes", needed, available)
	}

	return nil
}

func handleStorage(msgHandler *messages.MessageHandler, request *messages.StorageRequest) {
	fileName := filepath.Base(request.FileName)
	log.Println("Attempting to store", fileName)

	if err := ensureEnoughDiskSpace(".", request.Size); err != nil {
		msgHandler.SendResponse(false, err.Error())
		return
	}

	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		msgHandler.SendResponse(false, err.Error())
		return
	}
	defer file.Close()

	msgHandler.SendResponse(true, "Ready for data")
	md5 := md5.New()
	w := io.MultiWriter(file, md5)
	if _, err := io.CopyN(w, msgHandler, int64(request.Size)); err != nil {
		_ = os.Remove(fileName)
		msgHandler.SendResponse(false, fmt.Sprintf("failed receiving file data: %v", err))
		return
	}

	// CHANGE: verify checksum from PUT metadata; no post-stream checksum message.
	serverChecksum := md5.Sum(nil)
	if util.VerifyChecksum(serverChecksum, request.Checksum) {
		log.Println("Successfully stored file.")
		msgHandler.SendResponse(true, "Storage complete")
	} else {
		log.Println("FAILED to store file. Invalid checksum.")
		_ = os.Remove(fileName)
		msgHandler.SendResponse(false, "Checksum mismatch")
	}
}

func handleRetrieval(msgHandler *messages.MessageHandler, request *messages.RetrievalRequest) {
	fileName := filepath.Base(request.FileName)
	log.Println("Attempting to retrieve", fileName)

	// CHANGE: open file first so we never send OK then fail.
	file, err := os.Open(fileName)
	if err != nil {
		_ = msgHandler.SendRetrievalResponse(false, "File not found", 0, nil)
		return
	}
	defer file.Close()

	// CHANGE: stat after opening.
	info, err := file.Stat()
	if err != nil {
		log.Println("failed to stat file:", err)
		return
	}

	// CHANGE: compute checksum before sending retrieval metadata.
	h := md5.New()
	if _, err := io.Copy(h, file); err != nil {
		log.Println("failed to hash file:", err)
		return
	}
	checksum := h.Sum(nil)

	// CHANGE: rewind for streaming after checksum pass.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		log.Println("failed to rewind file:", err)
		return
	}

	// CHANGE: include checksum metadata in retrieval response.
	if err := msgHandler.SendRetrievalResponse(true, "Ready to send", uint64(info.Size()), checksum); err != nil {
		log.Println("failed to send retrieval response:", err)
		return
	}

	// CHANGE: stream raw bytes only; no post-stream checksum message.
	if _, err := io.CopyN(msgHandler, file, info.Size()); err != nil {
		log.Println("failed streaming file:", err)
		return
	}
}

func handleClient(msgHandler *messages.MessageHandler) {
	defer msgHandler.Close()

	for {
		wrapper, err := msgHandler.Receive()
		if err != nil {
			log.Println(err)
			return
		}

		switch msg := wrapper.Msg.(type) {
		case *messages.Wrapper_StorageReq:
			handleStorage(msgHandler, msg.StorageReq)
		case *messages.Wrapper_RetrievalReq:
			handleRetrieval(msgHandler, msg.RetrievalReq)
		case nil:
			log.Println("Received an empty message, terminating client")
			return
		default:
			log.Printf("Unexpected message type: %T", msg)
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Not enough arguments. Usage: %s port [download-dir]\n", os.Args[0])
		os.Exit(1)
	}

	port := os.Args[1]
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalln(err.Error())
		os.Exit(1)
	}
	defer listener.Close()

	dir := "."
	if len(os.Args) >= 3 {
		dir = os.Args[2]
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalln(err)
	}
	if err := os.Chdir(dir); err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Listening on port:", port)
	fmt.Println("Download directory:", dir)
	for {
		if conn, err := listener.Accept(); err == nil {
			log.Println("Accepted connection", conn.RemoteAddr())
			handler := messages.NewMessageHandler(conn)
			go handleClient(handler)
		}
	}
}
