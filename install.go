package xcdevice

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"

	"github.com/romantomjak/xcdevice/infoplist"
)

func Install(device *Device, filepath string) error {
	if _, err := os.Stat(filepath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("the file at %s does not exist. typo?\n", filepath)
			return err
		}
		return err
	}

	info, err := infoplist.Stat(filepath)
	if err != nil {
		return err
	}

	bundleID, ok := info[infoplist.PlistKeyBundleIdentifier].(string)
	if !ok {
		return fmt.Errorf("failed to cast BundleIdentifier")
	}

	lockdown, err := LockdownService(device)
	if err != nil {
		return fmt.Errorf("lockdown: %v", err)
	}

	afc, err := lockdown.AFCService()
	if err != nil {
		return fmt.Errorf("afc: %v", err)
	}

	stagingPath := "PublicStaging"
	pathInfo, err := afc.Stat(stagingPath)
	if err != nil {
		if !errors.Is(err, ErrObjectNotFound) {
			return err
		}
	}
	if pathInfo == nil {
		if err := afc.CreateDirectory(stagingPath); err != nil {
			return err
		}
	}

	installationPath := path.Join(stagingPath, fmt.Sprintf("%s.ipa", bundleID))

	bytes, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}

	if err := afc.WriteFile(installationPath, bytes, AfcFileModeWr); err != nil {
		return err
	}

	installationProxy, err := lockdown.InstallationProxyService()
	if err != nil {
		return fmt.Errorf("installation proxy: %v", err)
	}

	if err := installationProxy.InstallApplication(bundleID, installationPath); err != nil {
		return err
	}

	return nil
}
