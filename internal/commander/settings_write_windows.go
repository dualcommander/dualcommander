package commander

import "os"

func writeSettingsData(path string, data []byte) error {
	// ponytail: direct Windows writes avoid replacement/sharing failures; interrupted
	// writes may leave partial JSON. Use safe replacement if crash recovery is needed.
	return os.WriteFile(path, data, 0o600)
}
