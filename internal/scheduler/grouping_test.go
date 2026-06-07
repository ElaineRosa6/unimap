package scheduler

import (
	"slices"
	"testing"
)

// TestGroupedTaskTypes_CoversAllTypes verifies the UI groups partition exactly
// the full set of task types: every type appears once, with no extras.
func TestGroupedTaskTypes_CoversAllTypes(t *testing.T) {
	all := AllTaskTypes()
	seen := make(map[TaskType]int)
	for _, g := range GroupedTaskTypes() {
		if g.Name == "" {
			t.Error("group has empty name")
		}
		if g.Icon == "" {
			t.Errorf("group %q has empty icon", g.Name)
		}
		for _, tt := range g.Types {
			seen[tt]++
		}
	}

	for _, tt := range all {
		switch seen[tt] {
		case 0:
			t.Errorf("task type %q is not assigned to any group", tt)
		case 1:
			// ok
		default:
			t.Errorf("task type %q appears in %d groups (want 1)", tt, seen[tt])
		}
	}

	for tt := range seen {
		if !slices.Contains(all, tt) {
			t.Errorf("group references unknown task type %q", tt)
		}
	}

	if len(seen) != len(all) {
		t.Errorf("grouped %d distinct types, want %d", len(seen), len(all))
	}
}

// TestTaskTypeGroup_KnownAndUnknown verifies group lookup for known types and
// the fallback for an unknown type.
func TestTaskTypeGroup_KnownAndUnknown(t *testing.T) {
	tests := []struct {
		typ  TaskType
		want string
	}{
		{TaskQuery, "查询与采集"},
		{TaskTamperCheck, "监控与检测"},
		{TaskScreenshotCleanup, "维护与清理"},
		{TaskCookieVerify, "基础设施"},
		{TaskURLImport, "导入与汇总"},
		{TaskType("nonexistent"), "其他"},
	}
	for _, tc := range tests {
		if got := TaskTypeGroup(tc.typ); got != tc.want {
			t.Errorf("TaskTypeGroup(%q) = %q, want %q", tc.typ, got, tc.want)
		}
	}
}
