package main

import "testing"

func TestLogTextShowsSyncResume(t *testing.T) {
	overwrite := "Warning: Studio is overwriting 1 synced hierarchies in the DM when restarting to match the current contents in the disk."
	if !logTextShowsSyncResume(overwrite) {
		t.Fatal("overwrite warning should count as resume")
	}
	if logTextShowsSyncResume("Studio did not resume syncing because SafetyBit") {
		t.Fatal("did-not-resume must not count")
	}
	if !logTextShowsSyncResume(`ROBUILD_JSON:{"sync":{"roots":[{"status":"SyncedAsRoot"}]}}`) {
		t.Fatal("plugin SyncedAsRoot should count")
	}
}
