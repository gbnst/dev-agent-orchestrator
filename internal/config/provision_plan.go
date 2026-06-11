// pattern: Functional Core

package config

import (
	"bytes"
	"sort"
)

// TemplateSyncPlan describes how to reconcile the embedded template files with
// the copy currently materialized in the user's profile.
type TemplateSyncPlan struct {
	// Write lists template file paths (relative to the templates root, using
	// forward slashes) whose content should be written to disk.
	Write []string
	// Backup lists the subset of Write paths whose on-disk content differs from
	// the embedded content and must be backed up before being overwritten.
	Backup []string
}

// PlanTemplateSync computes the reconciliation between the embedded template
// files and the files currently on disk. Both maps are keyed by path relative
// to the templates root (forward slashes).
//
// For each embedded file:
//   - absent on disk        -> Write (no backup)
//   - present and identical -> skipped
//   - present but different -> Write + Backup
//
// Files present on disk but absent from the embedded set are left untouched:
// callers never delete user-provided templates. The plan is deterministic
// (paths sorted) so output and tests are stable.
func PlanTemplateSync(embedded, onDisk map[string][]byte) TemplateSyncPlan {
	keys := make([]string, 0, len(embedded))
	for k := range embedded {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	plan := TemplateSyncPlan{}
	for _, k := range keys {
		cur, ok := onDisk[k]
		if !ok {
			plan.Write = append(plan.Write, k)
			continue
		}
		if bytes.Equal(cur, embedded[k]) {
			continue
		}
		plan.Write = append(plan.Write, k)
		plan.Backup = append(plan.Backup, k)
	}
	return plan
}

// TemplatesNeedSync reports whether the materialized templates are stale for the
// running binary, given the version recorded in the profile marker. An empty
// marker (first run, or a pre-marker profile from an older devagent) always
// needs a sync.
func TemplatesNeedSync(markerVersion, binaryVersion string) bool {
	return markerVersion != binaryVersion
}
