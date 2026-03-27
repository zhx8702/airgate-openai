package gateway

import "testing"

func TestCodexUsageSnapshotNormalizeWithExplicitWindowMinutes(t *testing.T) {
	snapshot := &CodexUsageSnapshot{
		PrimaryUsedPercent:         70,
		PrimaryResetAfterSeconds:   86400,
		PrimaryWindowMinutes:       10080,
		SecondaryUsedPercent:       15,
		SecondaryResetAfterSeconds: 1200,
		SecondaryWindowMinutes:     300,
	}

	normalized := snapshot.Normalize()
	if normalized == nil {
		t.Fatalf("expected normalized limits")
	}
	if normalized.Used5hPercent == nil || *normalized.Used5hPercent != 15 {
		t.Fatalf("expected 5h usage to come from secondary, got %#v", normalized.Used5hPercent)
	}
	if normalized.Used7dPercent == nil || *normalized.Used7dPercent != 70 {
		t.Fatalf("expected 7d usage to come from primary, got %#v", normalized.Used7dPercent)
	}
}

func TestCodexUsageSnapshotNormalizeWithoutWindowMinutesUsesLegacyFallback(t *testing.T) {
	snapshot := &CodexUsageSnapshot{
		PrimaryUsedPercent:         50,
		PrimaryResetAfterSeconds:   10000,
		SecondaryUsedPercent:       20,
		SecondaryResetAfterSeconds: 3000,
	}

	normalized := snapshot.Normalize()
	if normalized == nil {
		t.Fatalf("expected normalized limits")
	}
	if normalized.Used5hPercent == nil || *normalized.Used5hPercent != 20 {
		t.Fatalf("expected 5h usage to come from secondary, got %#v", normalized.Used5hPercent)
	}
	if normalized.Used7dPercent == nil || *normalized.Used7dPercent != 50 {
		t.Fatalf("expected 7d usage to come from primary, got %#v", normalized.Used7dPercent)
	}
}
