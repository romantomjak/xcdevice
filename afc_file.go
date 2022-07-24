package xcdevice

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"sync/atomic"
)

type afcFile struct {
	conn      net.Conn
	packetNum uint64
	fd        uint64
}

func (f *afcFile) Write(b []byte) (int, error) {
	dataBuf := &bytes.Buffer{}
	if err := binary.Write(dataBuf, binary.LittleEndian, f.fd); err != nil {
		return 0, err
	}
	dataPayload := dataBuf.Bytes()

	n := uint64(len(dataPayload))

	var magic [8]byte
	copy(magic[:], "CFA6LPAA")

	req := afcOperationRequest{
		Magic:        magic,
		EntireLength: 40 + n + uint64(len(b)),
		ThisLength:   40 + n,
		PacketNum:    atomic.AddUint64(&f.packetNum, 1),
		Operation:    AfcOperationFileWrite,
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, req); err != nil {
		return 0, err
	}
	buf.Write(dataPayload)
	buf.Write(b)

	log.Printf(">> <file-bytes>\n")

	if n, err := f.conn.Write(buf.Bytes()); err != nil {
		return n, err
	}

	var respHeader afcOperationRequest
	if err := binary.Read(f.conn, binary.LittleEndian, &respHeader); err != nil {
		return 0, err
	}

	respData := make([]byte, respHeader.ThisLength-40)
	if _, err := io.ReadFull(f.conn, respData); err != nil {
		return 0, err
	}

	if respHeader.Operation == AfcOperationStatus {
		code := binary.LittleEndian.Uint64(respData)
		return 0, errorsToErrors[code]
	}

	respPayload := make([]byte, respHeader.EntireLength-respHeader.ThisLength)
	if _, err := io.ReadFull(f.conn, respPayload); err != nil {
		return 0, err
	}

	m := make(map[string]interface{}, 0)
	bs := bytes.Split(respPayload, []byte{0x00})
	for i := 0; i < len(bs); i += 2 {
		if i == len(bs)-1 {
			break
		}
		m[string(bs[i])] = string(bs[i+1])
	}

	log.Printf("<< %s\n", m)

	return len(b), nil
}

func (f *afcFile) Close() error {
	dataBuf := &bytes.Buffer{}
	if err := binary.Write(dataBuf, binary.LittleEndian, f.fd); err != nil {
		return err
	}
	dataPayload := dataBuf.Bytes()

	n := uint64(len(dataPayload))

	var magic [8]byte
	copy(magic[:], "CFA6LPAA")

	req := afcOperationRequest{
		Magic:        magic,
		EntireLength: 40 + n,
		ThisLength:   40 + n,
		PacketNum:    atomic.AddUint64(&f.packetNum, 1),
		Operation:    AfcOperationFileClose,
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, req); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, dataBuf); err != nil {
		return err
	}

	payload := buf.Bytes()

	log.Printf(">> %s\n", payload)

	if _, err := f.conn.Write(buf.Bytes()); err != nil {
		return err
	}

	var respHeader afcOperationRequest
	if err := binary.Read(f.conn, binary.LittleEndian, &respHeader); err != nil {
		return err
	}

	respData := make([]byte, respHeader.ThisLength-40)
	if _, err := io.ReadFull(f.conn, respData); err != nil {
		return err
	}

	if respHeader.Operation == AfcOperationStatus {
		code := binary.LittleEndian.Uint64(respData)
		return errorsToErrors[code]
	}

	respPayload := make([]byte, respHeader.EntireLength-respHeader.ThisLength)
	if _, err := io.ReadFull(f.conn, respPayload); err != nil {
		return err
	}

	m := make(map[string]interface{}, 0)
	bs := bytes.Split(respPayload, []byte{0x00})
	for i := 0; i < len(bs); i += 2 {
		if i == len(bs)-1 {
			break
		}
		m[string(bs[i])] = string(bs[i+1])
	}

	log.Printf("<< %s\n", m)

	return nil
}
