package xcdevice

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"
)

type AfcFileMode uint32

const (
	AfcFileModeRdOnly   AfcFileMode = 0x00000001
	AfcFileModeRw       AfcFileMode = 0x00000002
	AfcFileModeWrOnly   AfcFileMode = 0x00000003
	AfcFileModeWr       AfcFileMode = 0x00000004
	AfcFileModeAppend   AfcFileMode = 0x00000005
	AfcFileModeRdAppend AfcFileMode = 0x00000006
)

const (
	AfcOperationStatus         = 0x00000001
	AfcOperationWriteFile      = 0x00000005
	AfcOperationMakeDir        = 0x00000009
	AfcOperationGetFileInfo    = 0x0000000A
	AfcOperationFileOpen       = 0x0000000D
	AfcOperationFileOpenResult = 0x0000000E
	AfcOperationFileWrite      = 0x00000010
	AfcOperationFileClose      = 0x00000014
)

const AfcMagic uint64 = 0x4141504c36414643

const (
	afcESuccess             = 0
	afcEUnknownError        = 1
	afcEOpHeaderInvalid     = 2
	afcENoResources         = 3
	afcEReadError           = 4
	afcEWriteError          = 5
	afcEUnknownPacketType   = 6
	afcEInvalidArg          = 7
	afcEObjectNotFound      = 8
	afcEObjectIsDir         = 9
	afcEPermDenied          = 10
	afcEServiceNotConnected = 11
	afcEOpTimeout           = 12
	afcETooMuchData         = 13
	afcEEndOfData           = 14
	afcEOpNotSupported      = 15
	afcEObjectExists        = 16
	afcEObjectBusy          = 17
	afcENoSpaceLeft         = 18
	afcEOpWouldBlock        = 19
	afcEIoError             = 20
	afcEOpInterrupted       = 21
	afcEOpInProgress        = 22
	afcEInternalError       = 23
)

var (
	errorsToErrors = map[uint64]error{
		afcEUnknownError:        errors.New("unknown error"),
		afcEOpHeaderInvalid:     errors.New("invalid operation header"),
		afcENoResources:         errors.New("no resources"),
		afcEReadError:           errors.New("read error"),
		afcEWriteError:          errors.New("write error"),
		afcEUnknownPacketType:   errors.New("unknown packet type"),
		afcEInvalidArg:          errors.New("invalid argument"),
		afcEObjectNotFound:      ErrObjectNotFound,
		afcEObjectIsDir:         errors.New("object is a directory"),
		afcEPermDenied:          errors.New("permission denied"),
		afcEServiceNotConnected: errors.New("service not connected"),
		afcEOpTimeout:           errors.New("operation timeout"),
		afcETooMuchData:         errors.New("too much data"),
		afcEEndOfData:           io.EOF,
		afcEOpNotSupported:      errors.New("operation not supported"),
		afcEObjectExists:        errors.New("object exists"),
		afcEObjectBusy:          errors.New("object busy"),
		afcENoSpaceLeft:         errors.New("no space left"),
		afcEOpWouldBlock:        errors.New("operation would block"),
		afcEIoError:             errors.New("io error"),
		afcEOpInterrupted:       errors.New("operation interrupted"),
		afcEOpInProgress:        errors.New("operation in progress"),
		afcEInternalError:       errors.New("internal error"),
	}
)

var (
	ErrObjectNotFound = errors.New("object not found")
)

type afcOperationRequest struct {
	Magic        [8]byte
	EntireLength uint64
	ThisLength   uint64
	PacketNum    uint64
	Operation    uint64
}

type afcOperationResponse struct {
	Operation uint64
	Data      []byte
	Payload   []byte
}

type AFC struct {
	conn      net.Conn
	packetNum uint64
}

func (a *AFC) Stat(filepath string) (map[string]interface{}, error) {
	n := uint64(len(filepath))

	var magic [8]byte
	copy(magic[:], "CFA6LPAA")

	req := afcOperationRequest{
		Magic:        magic,
		EntireLength: 40 + uint64(n),
		ThisLength:   40 + uint64(n),
		PacketNum:    atomic.AddUint64(&a.packetNum, 1),
		Operation:    AfcOperationGetFileInfo,
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, req); err != nil {
		return nil, err
	}
	buf.WriteString(filepath)

	payload := buf.Bytes()

	log.Printf(">> %s\n", payload)

	if _, err := a.conn.Write(buf.Bytes()); err != nil {
		return nil, err
	}

	var respHeader afcOperationRequest
	if err := binary.Read(a.conn, binary.LittleEndian, &respHeader); err != nil {
		return nil, err
	}

	respData := make([]byte, respHeader.ThisLength-40)
	if _, err := io.ReadFull(a.conn, respData); err != nil {
		return nil, err
	}

	if respHeader.Operation == AfcOperationStatus {
		code := binary.LittleEndian.Uint64(respData)
		return nil, errorsToErrors[code]
	}

	respPayload := make([]byte, respHeader.EntireLength-respHeader.ThisLength)
	if _, err := io.ReadFull(a.conn, respPayload); err != nil {
		return nil, err
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

	return m, nil
}

func (a *AFC) WriteFile(filename string, data []byte, mode AfcFileMode) error {
	f, err := a.open(filename, mode)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = f.Write(data); err != nil {
		return err
	}

	return nil
}

func (a *AFC) CreateDirectory(name string) error {
	dataBuf := new(bytes.Buffer)
	dataBuf.WriteString(name)

	dataPayload := dataBuf.Bytes()

	n := uint64(len(dataPayload))

	var magic [8]byte
	copy(magic[:], "CFA6LPAA")

	req := afcOperationRequest{
		Magic:        magic,
		EntireLength: 40 + n,
		ThisLength:   40 + n,
		PacketNum:    atomic.AddUint64(&a.packetNum, 1),
		Operation:    AfcOperationMakeDir,
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, req); err != nil {
		return err
	}
	buf.Write(dataPayload)

	payload := buf.Bytes()

	log.Printf(">> %s\n", payload)

	if _, err := a.conn.Write(buf.Bytes()); err != nil {
		return err
	}

	var respHeader afcOperationRequest
	if err := binary.Read(a.conn, binary.LittleEndian, &respHeader); err != nil {
		return err
	}

	respData := make([]byte, respHeader.ThisLength-40)
	if _, err := io.ReadFull(a.conn, respData); err != nil {
		return err
	}

	if respHeader.Operation == AfcOperationStatus {
		code := binary.LittleEndian.Uint64(respData)
		return errorsToErrors[code]
	}

	respPayload := make([]byte, respHeader.EntireLength-respHeader.ThisLength)
	if _, err := io.ReadFull(a.conn, respPayload); err != nil {
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

func (a *AFC) open(filename string, mode AfcFileMode) (*afcFile, error) {
	dataBuf := new(bytes.Buffer)
	if err := binary.Write(dataBuf, binary.LittleEndian, uint64(mode)); err != nil {
		return nil, fmt.Errorf("afc open: %v", err)
	}
	dataBuf.WriteString(filename)

	dataPayload := dataBuf.Bytes()

	n := uint64(len(dataPayload))

	var magic [8]byte
	copy(magic[:], "CFA6LPAA")

	req := afcOperationRequest{
		Magic:        magic,
		EntireLength: 40 + n,
		ThisLength:   40 + n,
		PacketNum:    atomic.AddUint64(&a.packetNum, 1),
		Operation:    AfcOperationFileOpen,
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, req); err != nil {
		return nil, err
	}
	buf.Write(dataPayload)

	payload := buf.Bytes()

	log.Printf(">> %s\n", payload)

	if _, err := a.conn.Write(buf.Bytes()); err != nil {
		return nil, err
	}

	var respHeader afcOperationRequest
	if err := binary.Read(a.conn, binary.LittleEndian, &respHeader); err != nil {
		return nil, err
	}

	respData := make([]byte, respHeader.ThisLength-40)
	if _, err := io.ReadFull(a.conn, respData); err != nil {
		return nil, err
	}

	if respHeader.Operation != AfcOperationFileOpenResult {
		code := binary.LittleEndian.Uint64(respData)
		return nil, fmt.Errorf("open file: %s", errorsToErrors[code])
	}

	return &afcFile{a.conn, 0, binary.LittleEndian.Uint64(respData)}, nil
}
