package xcdevice

func Uninstall(device *Device, bundleID string) error {
	lockdown, err := LockdownService(device)
	if err != nil {
		return err
	}

	installationProxy, err := lockdown.InstallationProxyService()
	if err != nil {
		return err
	}

	if err := installationProxy.UninstallApplication(bundleID); err != nil {
		return err
	}

	return nil
}
