package admin

import "os"

// Every upload/import/update owns its temporary archive. Fixed names in /tmp
// let concurrent requests overwrite or remove each other's ZIP files.
func writeThemeArchive(data []byte) (string, error) {
	file, err := os.CreateTemp("", "komari-theme-*.zip")
	if err != nil {
		return "", err
	}
	name := file.Name()
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(name)
		return "", writeErr
	}
	if closeErr != nil {
		_ = os.Remove(name)
		return "", closeErr
	}
	return name, nil
}
