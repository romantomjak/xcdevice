package xcdevice

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"

	"howett.net/plist"
)

type ApplicationType string

const (
	ApplicationTypeAny      ApplicationType = "Any"
	ApplicationTypeSystem   ApplicationType = "System"
	ApplicationTypeUser     ApplicationType = "User"
	ApplicationTypeInternal ApplicationType = "Internal"
)

var DefaultLookupAttributes = []string{
	"CFBundleDisplayName",
	"CFBundleExecutable",
	"CFBundleName",
	"CFBundleVersion",
	"CFBundleShortVersionString",
	"CFBundleIdentifier",
}

type installationProxyInstallRequest struct {
	Command       string
	ClientOptions *installationProxyOption
	PackagePath   string
}

type installationProxyOption struct {
	ApplicationType       ApplicationType `plist:",omitempty"`
	ReturnAttributes      []string        `plist:",omitempty"`
	MetaData              bool            `plist:",omitempty"`
	BundleIDs             []string        `plist:",omitempty"`
	BundleID              string          `plist:",omitempty"`
	ApplicationIdentifier string          `plist:",omitempty"`
}

type installationProxyInstallResponse struct {
	Status           string
	Error            string
	ErrorDescription string
}

type installationProxyUninstallRequest struct {
	Command               string
	ClientOptions         *installationProxyOption `plist:",omitempty"`
	ApplicationIdentifier string
}

type installationProxyLookupRequest struct {
	Command       string
	ClientOptions *installationProxyOption
}

type installationProxyLookupResponse struct {
	Status       string
	LookupResult map[string]interface{}
}

type InstallationProxy struct {
	conn net.Conn
}

func (p *InstallationProxy) UninstallApplication(bundleID string) error {
	req := installationProxyUninstallRequest{
		Command:               "Uninstall",
		ApplicationIdentifier: bundleID,
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

	if _, err := p.conn.Write(buf.Bytes()); err != nil {
		return err
	}

	var resp installationProxyInstallResponse
	for len(resp.Error) == 0 {
		var respHeader header2
		if err := binary.Read(p.conn, binary.BigEndian, &respHeader); err != nil {
			return err
		}

		respPayload := make([]byte, respHeader.Length)
		if _, err := io.ReadFull(p.conn, respPayload); err != nil {
			return err
		}

		log.Printf("<< %s\n", respPayload)

		if _, err := plist.Unmarshal(respPayload, &resp); err != nil {
			return err
		}

		if resp.Status == "Complete" {
			break
		}
	}

	if len(resp.Error) != 0 {
		return fmt.Errorf("uninstall: %s (err: %s, desc: %s)", resp.Status, resp.Error, resp.ErrorDescription)
	}

	return nil
}

func (p *InstallationProxy) InstallApplication(bundleID, path string) error {
	req := installationProxyInstallRequest{
		Command: "Install",
		ClientOptions: &installationProxyOption{
			BundleID: bundleID,
		},
		PackagePath: path,
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

	if _, err := p.conn.Write(buf.Bytes()); err != nil {
		return err
	}

	var resp installationProxyInstallResponse
	for len(resp.Error) == 0 {
		var respHeader header2
		if err := binary.Read(p.conn, binary.BigEndian, &respHeader); err != nil {
			return err
		}

		respPayload := make([]byte, respHeader.Length)
		if _, err := io.ReadFull(p.conn, respPayload); err != nil {
			return err
		}

		log.Printf("<< %s\n", respPayload)

		if _, err := plist.Unmarshal(respPayload, &resp); err != nil {
			return err
		}

		if resp.Status == "Complete" {
			break
		}
	}

	if len(resp.Error) != 0 {
		return fmt.Errorf("install: %s (err: %s, desc: %s)", resp.Status, resp.Error, resp.ErrorDescription)
	}

	return nil
}

func (p *InstallationProxy) LookupApplication(bundleID string, attributes []string) (map[string]interface{}, error) {
	if len(attributes) == 0 {
		attributes = DefaultLookupAttributes
	}

	req := installationProxyLookupRequest{
		Command: "Lookup",
		ClientOptions: &installationProxyOption{
			BundleIDs:        []string{bundleID},
			ReturnAttributes: attributes,
			ApplicationType:  ApplicationTypeAny,
		},
	}

	payload, err := plist.Marshal(req, plist.XMLFormat)
	if err != nil {
		return nil, err
	}

	log.Printf(">> %s\n", payload)

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(len(payload)))

	buf := &bytes.Buffer{}

	buf.Write(b)
	buf.Write(payload)

	if _, err := p.conn.Write(buf.Bytes()); err != nil {
		return nil, err
	}

	var respHeader header2
	if err := binary.Read(p.conn, binary.BigEndian, &respHeader); err != nil {
		return nil, err
	}

	respPayload := make([]byte, respHeader.Length)
	if _, err := io.ReadFull(p.conn, respPayload); err != nil {
		return nil, err
	}

	log.Printf("<< %s\n", respPayload)

	var resp installationProxyLookupResponse
	if _, err := plist.Unmarshal(respPayload, &resp); err != nil {
		return nil, err
	}

	if resp.Status != "Complete" {
		return nil, fmt.Errorf("lookup status: %s", resp.Status)
	}

	data, ok := resp.LookupResult[bundleID].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("lookup error: %v", err)
	}

	return data, nil
}
