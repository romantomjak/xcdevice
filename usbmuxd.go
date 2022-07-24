package xcdevice

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	"howett.net/plist"
)

const (
	ConnectionTypeUSB = "USB"
)

type ReplyCode uint64

const (
	ReplyCodeOK ReplyCode = iota
	ReplyCodeBadCommand
	ReplyCodeBadDevice
	ReplyCodeConnectionRefused
	_ // ignore `4`
	_ // ignore `5`
	ReplyCodeBadVersion
)

func (rc ReplyCode) String() string {
	switch rc {
	case ReplyCodeOK:
		return "ok"
	case ReplyCodeBadCommand:
		return "bad command"
	case ReplyCodeBadDevice:
		return "bad device"
	case ReplyCodeConnectionRefused:
		return "connection refused"
	case ReplyCodeBadVersion:
		return "bad version"
	default:
		return "unknown reply code: " + strconv.Itoa(int(rc))
	}
}

type Connection struct {
	// tag will be incremented for each message, so that responses can
	// be correlated to requests
	tag uint32

	// conn is the underlying socket connection to the usbmuxd daemon.
	conn net.Conn
}

type header struct {
	// Length is the lenght of the message including the header
	Length uint32

	// Version is the protocol version. Defaults to 1
	Version uint32

	// Request defines the message type.
	Request uint32

	// Tag is used to correlate responses to requests. This field is incremented
	// for every message sent.
	Tag uint32
}

type message struct {
	Header  header
	Payload []byte
}

func Open() (*Connection, error) {
	conn, err := net.Dial("unix", "/var/run/usbmuxd")
	if err != nil {
		return nil, fmt.Errorf("usbmuxd: %v", err)
	}
	return &Connection{0, conn}, nil
}

func (c *Connection) Send(request interface{}) error {
	body, err := plist.Marshal(request, plist.XMLFormat)
	if err != nil {
		return fmt.Errorf("usbmuxd: %v", err)
	}

	c.tag++

	h := header{
		Length:  16 + uint32(len(body)),
		Request: 8, // FIXME: add consts
		Version: 1,
		Tag:     c.tag,
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, h); err != nil {
		return fmt.Errorf("usbmuxd: %v", err)
	}
	buf.Write(body)

	payload := buf.Bytes()

	log.Printf(">> %s\n", payload)

	for totalSent := 0; totalSent < len(payload); {
		sent, err := c.conn.Write(payload[totalSent:])
		if err != nil {
			return fmt.Errorf("usbmuxd: %v", err)
		}
		if sent == 0 {
			return nil
		}
		totalSent += sent
	}

	return nil
}

func (c *Connection) Receive(v interface{}) error {
	h := header{}
	if err := binary.Read(c.conn, binary.LittleEndian, &h); err != nil {
		return fmt.Errorf("usbmuxd: %v", err)
	}

	body := make([]byte, h.Length-16)
	if _, err := io.ReadFull(c.conn, body); err != nil {
		return fmt.Errorf("usbmuxd: %v", err)
	}

	log.Printf("<< %s\n", body)

	if _, err := plist.Unmarshal(body, v); err != nil {
		return fmt.Errorf("usbmuxd: %v", err)
	}

	return nil
}

func (c *Connection) Hijack() net.Conn {
	return c.conn
}

func (c *Connection) Close() error {
	return c.conn.Close()
}
