package messages

import (
	"encoding/binary"
	"log"
	"net"

	"google.golang.org/protobuf/proto"
)

type MessageHandler struct {
	conn net.Conn
}

func NewMessageHandler(conn net.Conn) *MessageHandler {
	m := &MessageHandler{
		conn: conn,
	}

	return m
}

func (m *MessageHandler) ReadN(buf []byte) error {
	bytesRead := uint64(0)
	for bytesRead < uint64(len(buf)) {
		n, err := m.conn.Read(buf[bytesRead:])
		if err != nil {
			return err
		}
		bytesRead += uint64(n)
	}
	return nil
}

func (m *MessageHandler) Read(p []byte) (n int, err error) {
	return m.conn.Read(p)
}

func (m *MessageHandler) Write(p []byte) (n int, err error) {
	return m.conn.Write(p)
}

func (m *MessageHandler) WriteN(buf []byte) error {
	bytesWritten := uint64(0)
	for bytesWritten < uint64(len(buf)) {
		n, err := m.conn.Write(buf[bytesWritten:])
		if err != nil {
			return err
		}
		bytesWritten += uint64(n)
	}
	return nil
}

func (m *MessageHandler) Send(wrapper *Wrapper) error {
	serialized, err := proto.Marshal(wrapper)
	if err != nil {
		return err
	}

	prefix := make([]byte, 8)
	binary.LittleEndian.PutUint64(prefix, uint64(len(serialized)))
	if err := m.WriteN(prefix); err != nil {
		return err
	}
	if err := m.WriteN(serialized); err != nil {
		return err
	}

	return nil
}

func (m *MessageHandler) Receive() (*Wrapper, error) {
	prefix := make([]byte, 8)
	if err := m.ReadN(prefix); err != nil {
		return nil, err
	}

	payloadSize := binary.LittleEndian.Uint64(prefix)
	payload := make([]byte, payloadSize)
	if err := m.ReadN(payload); err != nil {
		return nil, err
	}

	wrapper := &Wrapper{}
	err := proto.Unmarshal(payload, wrapper)
	return wrapper, err
}

func (m *MessageHandler) Close() {
	m.conn.Close()
}

// CHANGE: include checksum metadata in storage request.
func (m *MessageHandler) SendStorageRequest(fileName string, size uint64, checksum []byte) error {
	msg := StorageRequest{FileName: fileName, Size: size, Checksum: checksum}
	wrapper := &Wrapper{
		Msg: &Wrapper_StorageReq{StorageReq: &msg},
	}
	return m.Send(wrapper)
}

func (m *MessageHandler) SendRetrievalRequest(fileName string) error {
	msg := RetrievalRequest{FileName: fileName}
	wrapper := &Wrapper{
		Msg: &Wrapper_RetrievalReq{RetrievalReq: &msg},
	}
	return m.Send(wrapper)
}

func (m *MessageHandler) SendChecksumVerification(checksum []byte) error {
	checkMsg := ChecksumVerification{Checksum: checksum}
	checkWrapper := &Wrapper{
		Msg: &Wrapper_Checksum{Checksum: &checkMsg},
	}
	return m.Send(checkWrapper)
}

func (m *MessageHandler) SendResponse(ok bool, str string) error {
	msg := Response{Ok: ok, Message: str}
	wrapper := &Wrapper{
		Msg: &Wrapper_Response{Response: &msg},
	}

	return m.Send(wrapper)
}

// CHANGE: include checksum metadata in retrieval response.
func (m *MessageHandler) SendRetrievalResponse(ok bool, str string, size uint64, checksum []byte) error {
	resp := Response{Ok: ok, Message: str}
	msg := RetrievalResponse{Resp: &resp, Size: size, Checksum: checksum}
	wrapper := &Wrapper{
		Msg: &Wrapper_RetrievalResp{RetrievalResp: &msg},
	}

	return m.Send(wrapper)
}

func (m *MessageHandler) ReceiveResponse() (bool, string) {
	resp, err := m.Receive()
	if err != nil {
		return false, ""
	}

	response := resp.GetResponse()
	if response == nil {
		log.Println("Expected Response but received a different message type")
		return false, ""
	}

	log.Println(response.Message)
	return response.Ok, response.Message
}

// CHANGE: return checksum metadata that arrives before streamed file bytes.
func (m *MessageHandler) ReceiveRetrievalResponse() (bool, string, uint64, []byte) {
	resp, err := m.Receive()
	if err != nil {
		return false, "", 0, nil
	}

	retrievalResp := resp.GetRetrievalResp()
	if retrievalResp == nil || retrievalResp.GetResp() == nil {
		log.Println("Expected RetrievalResponse but received a different message type")
		return false, "", 0, nil
	}

	rr := retrievalResp.GetResp()
	log.Println(rr.Message)
	return rr.Ok, rr.Message, retrievalResp.Size, retrievalResp.Checksum
}
