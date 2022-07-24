package xcdevice

type Device struct {
	ConnectionSpeed int
	ConnectionType  string
	DeviceID        int
	LocationID      int
	ProductID       int
	SerialNumber    string
}

type listDevicesRequest struct {
	MessageType         string
	ProgName            string
	ClientVersionString string
}

type listDevicesResponse struct {
	DeviceList []deviceListItem
}

type deviceListItem struct {
	MessageType string
	Properties  Device
}

func ListDevices() ([]Device, error) {
	conn, err := Open()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req := listDevicesRequest{
		MessageType:         "ListDevices",
		ProgName:            "idevice",
		ClientVersionString: "idevice-0.0.1",
	}
	if err := conn.Send(req); err != nil {
		return nil, err
	}

	resp := listDevicesResponse{}
	if err := conn.Receive(&resp); err != nil {
		return nil, err
	}

	devices := make([]Device, 0, len(resp.DeviceList))
	for _, i := range resp.DeviceList {
		i := i
		devices = append(devices, i.Properties)
	}

	return devices, nil
}
