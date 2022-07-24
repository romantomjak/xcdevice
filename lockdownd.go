package xcdevice

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"

	"howett.net/plist"
)

type ServiceName string

const (
	ServiceNameInstallationProxy ServiceName = "com.apple.mobile.installation_proxy"
	ServiceNameAFC               ServiceName = "com.apple.afc"
)

// Lockdown is used to start services on the device.
//
// lockdownd uses a simple packet format where each packet is a 32-bit
// big-endian word indicating the size of the payload of the packet.
// The packets themselves are in XML plist format.
type Lockdown struct {
	conn          net.Conn
	tlsConn       *tls.Conn
	dev           *Device
	sessionID     string
	sslHandshakes int
}

type connectRequest struct {
	MessageType         string
	ProgName            string
	ClientVersionString string
	DeviceID            int
	PortNumber          uint16
}

type connectResponse struct {
	MessageType string
	Number      ReplyCode
}

type startSessionRequest struct {
	Label           string
	ProtocolVersion string
	Request         string
	HostID          string
	SystemBUID      string
}

type startSessionResponse struct {
	Request          string
	Error            string
	EnableSessionSSL bool
	SessionID        string
}

type stopSessionRequest struct {
	Label           string
	ProtocolVersion string
	Request         string
	SessionID       string
}

type stopSessionResponse struct {
	Request string
	Error   string
}

type startServiceRequest struct {
	Label           string
	ProtocolVersion string
	Request         string
	Service         string
}

type startServiceResponse struct {
	Request          string
	Error            string
	EnableServiceSSL bool
	Port             int
	Service          string
}

type header2 struct {
	// Length is the lenght of the message including the header
	Length uint32
}

func LockdownService(device *Device) (*Lockdown, error) {
	conn, err := Open()
	if err != nil {
		return nil, err
	}

	// Convert default lockdown port to network byte order
	// 62078 => 32498
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, 62078)
	port := binary.LittleEndian.Uint16(buf)

	req := connectRequest{
		MessageType:         "Connect",
		ProgName:            "idevice",
		ClientVersionString: "idevice-0.0.1",
		DeviceID:            device.DeviceID,
		PortNumber:          port,
	}
	if err := conn.Send(req); err != nil {
		return nil, err
	}

	resp := connectResponse{}
	if err := conn.Receive(&resp); err != nil {
		return nil, err
	}

	if resp.Number != ReplyCodeOK {
		return nil, fmt.Errorf("usbmuxd: %s", resp.Number.String())
	}

	return &Lockdown{conn.Hijack(), nil, device, "", 0}, nil
}

func (l *Lockdown) Conn() net.Conn {
	if l.tlsConn != nil {
		return l.tlsConn
	}
	return l.conn
}

func (l *Lockdown) startService(service ServiceName) (net.Conn, error) {
	log.Printf("startService %s\n", service)

	pair, err := ReadPairRecord(l.dev)
	if err != nil {
		return nil, fmt.Errorf("read pair: %v", err)
	}

	if err := l.startSession(pair); err != nil {
		return nil, fmt.Errorf("start session: %v", err)
	}

	dynamicPort, enableSSL, err := l.startDaemon(service)
	if err != nil {
		return nil, fmt.Errorf("start daemon: %v", err)
	}

	if err = l.stopSession(); err != nil {
		return nil, fmt.Errorf("stop session: %v", err)
	}

	conn, err := Open()
	if err != nil {
		return nil, err
	}
	// defer conn.Close()

	// Convert default lockdown port to network byte order
	// 62078 => 32498
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(dynamicPort))
	port := binary.LittleEndian.Uint16(buf)

	req := connectRequest{
		MessageType:         "Connect",
		ProgName:            "idevice",
		ClientVersionString: "idevice-0.0.1",
		DeviceID:            l.dev.DeviceID,
		PortNumber:          port,
	}
	if err := conn.Send(req); err != nil {
		return nil, err
	}

	resp := connectResponse{}
	if err := conn.Receive(&resp); err != nil {
		return nil, err
	}

	if resp.Number != ReplyCodeOK {
		return nil, fmt.Errorf("lockdownd: %s", resp.Number.String())
	}

	if enableSSL {
		// FIXME: maybe an abstraction for the connection can automatically
		// 		  bootstrap tls?
		return nil, fmt.Errorf("daemon requested ssl is unsupported")
	}

	return conn.Hijack(), nil
}

func (l *Lockdown) startDaemon(service ServiceName) (int, bool, error) {
	req := startServiceRequest{
		Label:           "com.idevice",
		ProtocolVersion: "2",
		Request:         "StartService",
		Service:         string(service),
	}

	payload, err := plist.Marshal(req, plist.XMLFormat)
	if err != nil {
		return 0, false, err
	}

	log.Printf(">> %s\n", payload)

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(len(payload)))

	buf := &bytes.Buffer{}

	buf.Write(b)
	buf.Write(payload)

	if _, err := l.Conn().Write(buf.Bytes()); err != nil {
		return 0, false, err
	}

	var respHeader header2
	if err := binary.Read(l.Conn(), binary.BigEndian, &respHeader); err != nil {
		return 0, false, err
	}

	respPayload := make([]byte, respHeader.Length)
	if _, err := io.ReadFull(l.Conn(), respPayload); err != nil {
		return 0, false, err
	}

	log.Printf("<< %s\n", respPayload)

	resp := &startServiceResponse{}
	if _, err := plist.Unmarshal(respPayload, resp); err != nil {
		return 0, false, err
	}

	return resp.Port, resp.EnableServiceSSL, nil
}

func (l *Lockdown) startSession(pair *PairRecord) error {
	log.Println("startSession")

	if l.sessionID != "" {
		if err := l.stopSession(); err != nil {
			return err
		}
	}

	req := startSessionRequest{
		Label:           "com.idevice",
		ProtocolVersion: "2",
		Request:         "StartSession",
		HostID:          pair.HostID,
		SystemBUID:      pair.SystemBUID,
	}

	payload, err := plist.Marshal(req, plist.XMLFormat)
	if err != nil {
		return err
	}

	log.Printf(">> %s\n", payload)

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(len(payload)))

	buf := &bytes.Buffer{}

	buf.Write(b)
	buf.Write(payload)

	if _, err := l.Conn().Write(buf.Bytes()); err != nil {
		return err
	}

	var respHeader header2
	if err := binary.Read(l.Conn(), binary.BigEndian, &respHeader); err != nil {
		return err
	}

	respPayload := make([]byte, respHeader.Length)
	if _, err := io.ReadFull(l.Conn(), respPayload); err != nil {
		return err
	}

	log.Printf("<< %s\n", respPayload)

	resp := &startSessionResponse{}
	if _, err := plist.Unmarshal(respPayload, resp); err != nil {
		return err
	}

	if resp.EnableSessionSSL {
		if err = l.enableSSL(pair); err != nil {
			return fmt.Errorf("enable ssl: %v", err)
		}
	}

	l.sessionID = resp.SessionID

	return nil
}

func (l *Lockdown) stopSession() error {
	log.Printf("stopSession %q\n", l.sessionID)

	if l.sessionID == "" {
		return nil
	}

	req := stopSessionRequest{
		Label:           "com.idevice",
		ProtocolVersion: "2",
		Request:         "StopSession",
		SessionID:       l.sessionID,
	}

	payload, err := plist.Marshal(req, plist.XMLFormat)
	if err != nil {
		return err
	}

	log.Printf(">> %s\n", payload)

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(len(payload)))

	buf := &bytes.Buffer{}

	buf.Write(b)
	buf.Write(payload)

	if _, err := l.Conn().Write(buf.Bytes()); err != nil {
		return err
	}

	var respHeader header2
	if err := binary.Read(l.Conn(), binary.BigEndian, &respHeader); err != nil {
		return err
	}

	respPayload := make([]byte, respHeader.Length)
	if _, err := io.ReadFull(l.Conn(), respPayload); err != nil {
		return err
	}

	log.Printf("<< %s\n", respPayload)

	resp := &stopSessionResponse{}
	if _, err := plist.Unmarshal(respPayload, resp); err != nil {
		return err
	}

	l.sessionID = ""

	return nil
}

func (l *Lockdown) enableSSL(pair *PairRecord) error {
	minVersion := uint16(tls.VersionTLS11)
	maxVersion := uint16(tls.VersionTLS13)

	cert, err := tls.X509KeyPair(pair.RootCertificate, pair.RootPrivateKey)
	if err != nil {
		return fmt.Errorf("x509: %v", err)
	}

	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
		MinVersion:         minVersion,
		MaxVersion:         maxVersion,
	}

	l.tlsConn = tls.Client(l.conn, config)

	if err = l.tlsConn.Handshake(); err != nil {
		return fmt.Errorf("tls handshake: %v", err)
	}

	return nil
}

func (l *Lockdown) InstallationProxyService() (*InstallationProxy, error) {
	conn, err := l.startService(ServiceNameInstallationProxy)
	if err != nil {
		return nil, err
	}
	return &InstallationProxy{conn}, nil
}

func (l *Lockdown) AFCService() (*AFC, error) {
	conn, err := l.startService(ServiceNameAFC)
	if err != nil {
		return nil, err
	}
	return &AFC{conn, 0}, nil
}
