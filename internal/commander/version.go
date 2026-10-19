package commander

import "runtime/debug"

// buildVersion is set by Make and CI; plain Go builds use embedded VCS metadata.
var buildVersion string

func applicationVersion() string {
	if buildVersion != "" {
		return buildVersion
	}
	revision, dirty := "unknown", false
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
				if len(revision) > 12 {
					revision = revision[:12]
				}
			case "vcs.modified":
				dirty = setting.Value == "true"
			}
		}
	}
	if dirty {
		revision += ".dirty"
	}
	return appVersion + "+" + revision
}
