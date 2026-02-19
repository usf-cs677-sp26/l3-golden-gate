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
	"strings"
	"time"
)

// CHANGE: compute PUT checksum before sending metadata request.
func computeFileChecksum(fileName string) ([]byte, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func put(msgHandler *messages.MessageHandler, fileName string) int {
	fmt.Println("PUT", fileName)
	start := time.Now()

	// Get file size and make sure it exists
	info, err := os.Stat(fileName)
	if err != nil {
		log.Println(err)
		return 1
	}

	// CHANGE: include checksum in metadata before sending PUT bytes.
	checksum, err := computeFileChecksum(fileName)
	if err != nil {
		log.Println(err)
		return 1
	}

	if err := msgHandler.SendStorageRequest(filepath.Base(fileName), uint64(info.Size()), checksum); err != nil {
		log.Println(err)
		return 1
	}
	if ok, _ := msgHandler.ReceiveResponse(); !ok {
		return 1
	}

	file, err := os.Open(fileName)
	if err != nil {
		log.Println(err)
		return 1
	}
	defer file.Close()

	// CHANGE: stream bytes directly and rely on final server response.
	if _, err := io.CopyN(msgHandler, file, info.Size()); err != nil {
		log.Println(err)
		return 1
	}

	if ok, _ := msgHandler.ReceiveResponse(); !ok {
		return 1
	}

	fmt.Println("Storage complete!")
	elapsed := time.Since(start).Seconds()
	log.Printf("PUT completed in %.3f seconds\n", elapsed)
	return 0
}

func get(msgHandler *messages.MessageHandler, fileName string, destinationDir string) int {
	fmt.Println("GET", fileName)
	start := time.Now()

	remoteName := filepath.Base(fileName)
	if err := msgHandler.SendRetrievalRequest(remoteName); err != nil {
		log.Println(err)
		return 1
	}
	// CHANGE: retrieval metadata now includes message + checksum.
	ok, msg, size, expectedChecksum := msgHandler.ReceiveRetrievalResponse()
	if !ok {
		log.Println(msg)
		return 1
	}

	outputPath := filepath.Join(destinationDir, remoteName)
	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		log.Println(err)
		return 1
	}
	defer file.Close()

	// CHANGE: compute client checksum while streaming bytes to disk.
	md5 := md5.New()
	w := io.MultiWriter(file, md5)
	if _, err := io.CopyN(w, msgHandler, int64(size)); err != nil {
		log.Println(err)
		_ = os.Remove(outputPath)
		return 1
	}

	// CHANGE: verify against checksum from retrieval metadata; no trailing checksum message.
	clientCheck := md5.Sum(nil)
	if util.VerifyChecksum(expectedChecksum, clientCheck) {
		elapsed := time.Since(start).Seconds()
		log.Printf("GET completed in %.3f seconds\n", elapsed)
		log.Println("Successfully retrieved file:", outputPath)
		return 0
	} else {
		// CHANGE: remove output file on checksum mismatch.
		log.Println("FAILED to retrieve file. Invalid checksum.")
		_ = os.Remove(outputPath)
		return 1
	}
}

func main() {
	if len(os.Args) < 4 {
		fmt.Printf("Not enough arguments. Usage: %s server:port put|get file-name [download-dir]\n", os.Args[0])
		os.Exit(1)
	}

	host := os.Args[1]
	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Fatalln(err.Error())
		return
	}
	msgHandler := messages.NewMessageHandler(conn)
	defer conn.Close()

	action := strings.ToLower(os.Args[2])
	if action != "put" && action != "get" {
		log.Fatalln("Invalid action", action)
	}

	fileName := os.Args[3]

	if action == "put" {
		os.Exit(put(msgHandler, fileName))
	} else if action == "get" {
		destinationDir := "."
		if len(os.Args) >= 5 {
			destinationDir = os.Args[4]
		}

		info, err := os.Stat(destinationDir)
		if err != nil {
			log.Fatalln(err)
		}
		if !info.IsDir() {
			log.Fatalln("destination is not a directory:", destinationDir)
		}

		os.Exit(get(msgHandler, fileName, destinationDir))
	}
}
