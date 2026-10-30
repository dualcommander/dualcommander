package windows

import "embed"

// Licenses contains the same notices shipped beside the Windows runtime.
//
//go:embed mesa/*.txt runtime.json
var Licenses embed.FS
