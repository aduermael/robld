package main

type restartNeeds struct {
	Prefs  bool
	Plugin bool
	Resume bool
}

// studioNeedsRestart is true only when Studio is running and Script Sync
// prefs, the plugin, or resume records (including UniqueIds they need)
// actually require a write. Routine re-runs while already bound must not quit.
func studioNeedsRestart(running bool, n restartNeeds) bool {
	if !running {
		return false
	}
	return n.Prefs || n.Plugin || n.Resume
}
