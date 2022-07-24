package infoplist

import (
	"archive/zip"
	"fmt"
	"io"
	"path"

	"howett.net/plist"
)

const (
	PlistKeyBundleExecutable         = "CFBundleName"
	PlistKeyBundleIdentifier         = "CFBundleIdentifier"
	PlistKeyBundleShortVersionString = "CFBundleShortVersionString"
)

// Stat returns the Info.plist metadata for the specified IPA file.
func Stat(filename string) (map[string]interface{}, error) {
	zf, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer zf.Close()

	for _, file := range zf.File {
		matched, err := path.Match("Payload/*.app/Info.plist", file.Name)
		if err != nil {
			return nil, err
		}

		if !matched {
			continue
		}

		f, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer f.Close()

		bytes, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}

		data := make(map[string]interface{}, 0)
		if _, err := plist.Unmarshal(bytes, &data); err != nil {
			return nil, err
		}

		return data, nil
	}

	return nil, fmt.Errorf("missing Info.plist")
}
