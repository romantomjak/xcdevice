package xcdevice

import (
	"log"

	"howett.net/plist"
)

type readPairRecordRequest struct {
	MessageType         string
	ProgName            string
	ClientVersionString string
	PairRecordID        string
}

type readPairRecordResponse struct {
	PairRecordData []byte
}

type PairRecord struct {
	HostID            string
	SystemBUID        string
	HostCertificate   []byte
	HostPrivateKey    []byte
	DeviceCertificate []byte
	EscrowBag         []byte
	WiFiMACAddress    string
	RootCertificate   []byte
	RootPrivateKey    []byte
}

func ReadPairRecord(device *Device) (*PairRecord, error) {
	log.Printf("ReadPairRecord %s\n", device.SerialNumber)

	conn, err := Open()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req := readPairRecordRequest{
		MessageType:         "ReadPairRecord",
		ProgName:            "xcdevice",
		ClientVersionString: "xcdevice-0.0.1",
		PairRecordID:        device.SerialNumber,
	}
	if err := conn.Send(req); err != nil {
		return nil, err
	}

	resp := readPairRecordResponse{}
	if err := conn.Receive(&resp); err != nil {
		return nil, err
	}

	record := &PairRecord{}
	if _, err := plist.Unmarshal(resp.PairRecordData, record); err != nil {
		return nil, err
	}

	return record, nil
}
