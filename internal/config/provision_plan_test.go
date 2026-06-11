package config

import (
	"reflect"
	"testing"
)

func TestPlanTemplateSync(t *testing.T) {
	tests := []struct {
		name       string
		embedded   map[string][]byte
		onDisk     map[string][]byte
		wantWrite  []string
		wantBackup []string
	}{
		{
			name:       "absent on disk is written without backup",
			embedded:   map[string][]byte{"a/x.tmpl": []byte("1"), "b/y.tmpl": []byte("2")},
			onDisk:     map[string][]byte{},
			wantWrite:  []string{"a/x.tmpl", "b/y.tmpl"},
			wantBackup: nil,
		},
		{
			name:       "identical on disk is skipped",
			embedded:   map[string][]byte{"a/x.tmpl": []byte("same")},
			onDisk:     map[string][]byte{"a/x.tmpl": []byte("same")},
			wantWrite:  nil,
			wantBackup: nil,
		},
		{
			name:       "divergent on disk is written and backed up",
			embedded:   map[string][]byte{"a/x.tmpl": []byte("new")},
			onDisk:     map[string][]byte{"a/x.tmpl": []byte("old")},
			wantWrite:  []string{"a/x.tmpl"},
			wantBackup: []string{"a/x.tmpl"},
		},
		{
			name:       "extra on-disk files are left untouched",
			embedded:   map[string][]byte{"a/x.tmpl": []byte("1")},
			onDisk:     map[string][]byte{"a/x.tmpl": []byte("1"), "mine/z.tmpl": []byte("keep")},
			wantWrite:  nil,
			wantBackup: nil,
		},
		{
			name: "mixed: new, identical, divergent",
			embedded: map[string][]byte{
				"new.tmpl":     []byte("n"),
				"same.tmpl":    []byte("s"),
				"changed.tmpl": []byte("after"),
			},
			onDisk: map[string][]byte{
				"same.tmpl":    []byte("s"),
				"changed.tmpl": []byte("before"),
			},
			wantWrite:  []string{"changed.tmpl", "new.tmpl"}, // sorted
			wantBackup: []string{"changed.tmpl"},
		},
		{
			name:       "empty embedded yields empty plan",
			embedded:   map[string][]byte{},
			onDisk:     map[string][]byte{"a/x.tmpl": []byte("1")},
			wantWrite:  nil,
			wantBackup: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlanTemplateSync(tt.embedded, tt.onDisk)
			if !reflect.DeepEqual(got.Write, tt.wantWrite) {
				t.Errorf("Write = %v, want %v", got.Write, tt.wantWrite)
			}
			if !reflect.DeepEqual(got.Backup, tt.wantBackup) {
				t.Errorf("Backup = %v, want %v", got.Backup, tt.wantBackup)
			}
		})
	}
}

func TestTemplatesNeedSync(t *testing.T) {
	tests := []struct {
		marker, version string
		want            bool
	}{
		{"", "1.2.0", true},       // first run / pre-marker profile
		{"1.1.0", "1.2.0", true},  // upgrade
		{"1.2.0", "1.2.0", false}, // current
		{"dev", "dev", false},     // dev build, already synced
		{"1.2.0", "", true},       // defensive: empty binary version differs
	}
	for _, tt := range tests {
		if got := TemplatesNeedSync(tt.marker, tt.version); got != tt.want {
			t.Errorf("TemplatesNeedSync(%q, %q) = %v, want %v", tt.marker, tt.version, got, tt.want)
		}
	}
}
