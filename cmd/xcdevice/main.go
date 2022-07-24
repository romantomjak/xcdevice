package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/romantomjak/xcdevice"
)

var (
	deviceUUID string
	debug      bool
)

func init() {
	// Optionally specify the device by UDID. If not specified or empty
	// then the first USB device is selected.
	flag.StringVar(&deviceUUID, "device", "", "UDID of the device")

	// Enables verbose output.
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
}

func printUsage() {
	fmt.Printf(`Usage:
  xcdevice [flags] [command] [arguments]

Available Commands:
  install     Install application using an IPA file
  list        List all devices
  lookup	  Lookup application data by bundle ID
  uninstall   Uninstall application by bundle ID

Flags:
  --device string     specify a device using UDID (default "")
  --debug             enable verbose output (default false)
`)
}

func main() {
	flag.Parse()

	if !debug {
		log.SetOutput(io.Discard)
	}

	switch flag.Arg(0) {
	case "install":
		if flag.Arg(1) == "" {
			printUsage()
			os.Exit(1)
		}

		iphone, err := getDeviceByUDIDOrTakeFirst(deviceUUID)
		if err != nil {
			fmt.Printf("failed to get device: %v\n", err)
			os.Exit(1)
		}
		if iphone == nil {
			fmt.Println("no devices found. is the iphone plugged in?")
			os.Exit(1)
		}

		if err := xcdevice.Install(iphone, flag.Arg(1)); err != nil {
			fmt.Printf("installation error: %v\n", err)
			os.Exit(1)
		}

		os.Exit(0)

	case "list":
		devices, err := xcdevice.ListDevices()
		if err != nil {
			fmt.Printf("failed to list devices: %v\n", err)
			os.Exit(1)
		}

		// dedupe by device's UDID
		deviceMap := make(map[string]bool)
		for _, d := range devices {
			deviceMap[d.SerialNumber] = true
		}

		for k := range deviceMap {
			fmt.Printf("%s\n", k)
		}

		os.Exit(0)

	case "lookup":
		if flag.Arg(1) == "" {
			printUsage()
			os.Exit(1)
		}

		iphone, err := getDeviceByUDIDOrTakeFirst(deviceUUID)
		if err != nil {
			fmt.Printf("failed to get device: %v\n", err)
			os.Exit(1)
		}
		if iphone == nil {
			fmt.Println("no devices found. is the iphone plugged in?")
			os.Exit(1)
		}

		info, err := xcdevice.Lookup(iphone, flag.Arg(1), xcdevice.DefaultLookupAttributes)
		if err != nil {
			fmt.Printf("lookup error: %v\n", err)
			os.Exit(1)
		}

		for k, v := range info {
			fmt.Printf("%s: %v\n", k, v)
		}

		os.Exit(0)

	case "uninstall":
		if flag.Arg(1) == "" {
			printUsage()
			os.Exit(1)
		}

		iphone, err := getDeviceByUDIDOrTakeFirst(deviceUUID)
		if err != nil {
			fmt.Printf("failed to get device: %v\n", err)
			os.Exit(1)
		}
		if iphone == nil {
			fmt.Println("no devices found. is the iphone plugged in?")
			os.Exit(1)
		}

		if err := xcdevice.Uninstall(iphone, flag.Arg(1)); err != nil {
			fmt.Printf("uninstallation error: %v\n", err)
			os.Exit(1)
		}

		os.Exit(0)

	default:
		printUsage()
		os.Exit(1)
	}
}

func getDeviceByUDIDOrTakeFirst(udid string) (*xcdevice.Device, error) {
	devices, err := xcdevice.ListDevices()
	if err != nil {
		return nil, fmt.Errorf("list devices: %v", err)
	}

	// find the usb device with the specified uuid or if it was not specified
	// just return the first usb device. if the uuid is empty, we should ask
	// which device to use, but we'll fix it some other time. same goes for
	// network attached devices.
	for _, d := range devices {
		if d.ConnectionType == xcdevice.ConnectionTypeUSB {
			if udid == "" || d.SerialNumber == udid {
				return &d, nil
			}
		}
	}

	return nil, nil
}
