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

func put(msgHandler *messages.MessageHandler, fileName string) int {
	fmt.Println("PUT", fileName)

	// Get file size and make sure it exists
	info, err := os.Stat(fileName)
	if err != nil {
		log.Fatalln(err)
	}

	// Tell the server we want to store this file (send only the base name)
	msgHandler.SendStorageRequest(filepath.Base(fileName), uint64(info.Size()))
	if ok, _ := msgHandler.ReceiveResponse(); !ok {
		return 1
	}

	file, _ := os.Open(fileName)
	md5 := md5.New()
	w := io.MultiWriter(msgHandler, md5)
	start := time.Now()
	io.CopyN(w, file, info.Size()) // Checksum and transfer file at same time
	elapsed := time.Since(start)
	file.Close()

	checksum := md5.Sum(nil)
	msgHandler.SendChecksumVerification(checksum)
	if ok, _ := msgHandler.ReceiveResponse(); !ok {
		return 1
	}

	mbTransferred := float64(info.Size()) / 1_000_000.0
	throughput := mbTransferred / elapsed.Seconds()
	fmt.Printf("Storage complete! Transferred %.2f MB in %v (%.2f MB/s)\n", mbTransferred, elapsed, throughput)
	return 0
}

func get(msgHandler *messages.MessageHandler, fileName string, dir string) int {
	fmt.Println("GET", fileName)

	file, err := os.OpenFile(filepath.Join(dir, fileName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		log.Println(err)
		return 1
	}

	msgHandler.SendRetrievalRequest(fileName)
	ok, _, size := msgHandler.ReceiveRetrievalResponse()
	if !ok {
		return 1
	}

	md5 := md5.New()
	w := io.MultiWriter(file, md5)
	start := time.Now()
	io.CopyN(w, msgHandler, int64(size))
	elapsed := time.Since(start)
	file.Close()

	clientCheck := md5.Sum(nil)
	checkMsg, _ := msgHandler.Receive()
	serverCheck := checkMsg.GetChecksum().Checksum

	mbTransferred := float64(size) / 1_000_000.0
	throughput := mbTransferred / elapsed.Seconds()
	if util.VerifyChecksum(serverCheck, clientCheck) {
		fmt.Printf("Successfully retrieved file. Transferred %.2f MB in %v (%.2f MB/s)\n", mbTransferred, elapsed, throughput)
	} else {
		log.Println("FAILED to retrieve file. Invalid checksum.")
	}

	return 0
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

	dir := "."
	if len(os.Args) >= 5 {
		dir = os.Args[4]
	}
	openDir, err := os.Open(dir)
	if err != nil {
		log.Fatalln(err)
	}
	openDir.Close()

	if action == "put" {
		os.Exit(put(msgHandler, fileName))
	} else if action == "get" {
		os.Exit(get(msgHandler, fileName, dir))
	}
}
