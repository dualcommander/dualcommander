// Package assets embeds DualCommander's runtime app icon.
package assets

import _ "embed"

// IconPNG is the app icon shared by all desktop platforms.
//
//go:embed dualcommander.png
var IconPNG []byte
