package app

import (
	"testing"
	"time"
)

func TestUploadPolicy(t *testing.T) {
	limits, err := uploadPolicy(func(string) string { return "" })
	if err != nil || limits != (uploadLimits{quotaBytes: 1 << 30, quotaFiles: 1000, unusedAfter: 24 * time.Hour}) {
		t.Fatal("defaults", limits, err)
	}
	env := map[string]string{"IRCORD_UPLOAD_QUOTA_BYTES": "0", "IRCORD_UPLOAD_QUOTA_FILES": "50", "IRCORD_UPLOAD_UNUSED_RETENTION": "0"}
	limits, err = uploadPolicy(func(name string) string { return env[name] })
	if err != nil || limits != (uploadLimits{quotaFiles: 50}) {
		t.Fatal("zero turns a limit or the cleanup off", limits, err)
	}
	for name, value := range map[string]string{"IRCORD_UPLOAD_QUOTA_BYTES": "-1", "IRCORD_UPLOAD_QUOTA_FILES": "many", "IRCORD_UPLOAD_UNUSED_RETENTION": "30m"} {
		if _, err = uploadPolicy(func(n string) string {
			if n == name {
				return value
			}
			return ""
		}); err == nil {
			t.Fatalf("%s=%s accepted", name, value)
		}
	}
}
