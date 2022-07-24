package xcdevice

import (
	"fmt"
)

func Lookup(device *Device, bundleID string, attributes []string) (map[string]interface{}, error) {
	lockdown, err := LockdownService(device)
	if err != nil {
		return nil, fmt.Errorf("lockdown: %v", err)
	}

	installationProxy, err := lockdown.InstallationProxyService()
	if err != nil {
		return nil, fmt.Errorf("installation proxy: %v", err)
	}

	info, err := installationProxy.LookupApplication(bundleID, attributes)
	if err != nil {
		return nil, err
	}

	return info, nil
}
