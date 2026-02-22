package tsnsrv

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"devagent/internal/config"
)

func identity(s string) string { return s }

func TestBuildProcessConfigWith_Defaults(t *testing.T) {
	tc := config.TailscaleConfig{
		Name:        "devagent",
		Ephemeral:   true,
		AuthKeyPath: "/home/user/.config/devagent/tailscale-authkey",
		StateDir:    "/home/user/.local/share/devagent/tsnsrv",
	}

	pc, err := BuildProcessConfigWith(tc, "127.0.0.1:9001", identity, "/usr/bin/tsnsrv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pc.Name != "tsnsrv" {
		t.Errorf("Name = %q, want %q", pc.Name, "tsnsrv")
	}
	if pc.Binary != "/usr/bin/tsnsrv" {
		t.Errorf("Binary = %q, want %q", pc.Binary, "/usr/bin/tsnsrv")
	}

	expected := []string{
		"-name", "devagent",
		"-ephemeral",
		"-authkeyPath", "/home/user/.config/devagent/tailscale-authkey",
		"-stateDir", "/home/user/.local/share/devagent/tsnsrv",
		"http://127.0.0.1:9001",
	}

	if len(pc.Args) != len(expected) {
		t.Fatalf("Args length = %d, want %d\ngot:  %v\nwant: %v", len(pc.Args), len(expected), pc.Args, expected)
	}
	for i, arg := range expected {
		if pc.Args[i] != arg {
			t.Errorf("Args[%d] = %q, want %q", i, pc.Args[i], arg)
		}
	}
}

func TestBuildProcessConfigWith_Funnel(t *testing.T) {
	tc := config.TailscaleConfig{
		Name:      "myapp",
		Funnel:    true,
		Ephemeral: false,
	}

	pc, err := BuildProcessConfigWith(tc, "0.0.0.0:8080", identity, "/bin/tsnsrv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, pc.Args, "-funnel")
	assertNotContains(t, pc.Args, "-ephemeral")
	assertContains(t, pc.Args, "http://0.0.0.0:8080")
}

func TestBuildProcessConfigWith_FunnelOnly(t *testing.T) {
	tc := config.TailscaleConfig{
		Name:       "myapp",
		Funnel:     true,
		FunnelOnly: true,
	}

	pc, err := BuildProcessConfigWith(tc, "127.0.0.1:9001", identity, "/bin/tsnsrv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, pc.Args, "-funnel")
	assertContains(t, pc.Args, "-funnelOnly")
}

func TestBuildProcessConfigWith_Plaintext(t *testing.T) {
	tc := config.TailscaleConfig{
		Name:      "myapp",
		Plaintext: true,
	}

	pc, err := BuildProcessConfigWith(tc, "127.0.0.1:9001", identity, "/bin/tsnsrv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, pc.Args, "-plaintext")
}

func TestBuildProcessConfigWith_Tags(t *testing.T) {
	tc := config.TailscaleConfig{
		Name: "myapp",
		Tags: []string{"tag:devagent", "tag:server"},
	}

	pc, err := BuildProcessConfigWith(tc, "127.0.0.1:9001", identity, "/bin/tsnsrv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Tags should appear as -tag tag:devagent -tag tag:server
	tagCount := 0
	for i, arg := range pc.Args {
		if arg == "-tag" {
			tagCount++
			if i+1 >= len(pc.Args) {
				t.Fatal("-tag flag without value")
			}
		}
	}
	if tagCount != 2 {
		t.Errorf("expected 2 -tag flags, got %d in %v", tagCount, pc.Args)
	}
}

func TestBuildProcessConfigWith_PathResolution(t *testing.T) {
	tc := config.TailscaleConfig{
		Name:        "test",
		AuthKeyPath: "~/keys/authkey",
		StateDir:    "~/state/tsnsrv",
	}

	resolver := func(s string) string {
		if s == "~/keys/authkey" {
			return "/home/user/keys/authkey"
		}
		if s == "~/state/tsnsrv" {
			return "/home/user/state/tsnsrv"
		}
		return s
	}

	pc, err := BuildProcessConfigWith(tc, "127.0.0.1:9001", resolver, "/bin/tsnsrv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContainsSeq(t, pc.Args, "-authkeyPath", "/home/user/keys/authkey")
	assertContainsSeq(t, pc.Args, "-stateDir", "/home/user/state/tsnsrv")
}

func assertContains(t *testing.T, args []string, want string) {
	t.Helper()
	for _, a := range args {
		if a == want {
			return
		}
	}
	t.Errorf("args %v does not contain %q", args, want)
}

func assertNotContains(t *testing.T, args []string, unwanted string) {
	t.Helper()
	for _, a := range args {
		if a == unwanted {
			t.Errorf("args %v should not contain %q", args, unwanted)
			return
		}
	}
}

func assertContainsSeq(t *testing.T, args []string, key, value string) {
	t.Helper()
	for i, a := range args {
		if a == key && i+1 < len(args) && args[i+1] == value {
			return
		}
	}
	t.Errorf("args %v does not contain %q %q sequence", args, key, value)
}

// writeStateFileWithKey creates a state file where the map key differs from the Key field.
// This matches the real tailscale state format (e.g. map key "4242", Key "profile-4242").
func writeStateFileWithKey(t *testing.T, dir, currentProfile, mapKey, fqdn string) {
	t.Helper()

	profiles := map[string]struct {
		Name string `json:"Name"`
		Key  string `json:"Key"`
	}{
		mapKey: {Name: fqdn, Key: currentProfile},
	}
	profilesJSON, err := json.Marshal(profiles)
	if err != nil {
		t.Fatal(err)
	}

	state := map[string]string{
		"_current-profile": base64.StdEncoding.EncodeToString([]byte(currentProfile)),
		"_profiles":        base64.StdEncoding.EncodeToString(profilesJSON),
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tailscaled.state"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

// writeStateFile creates a valid tailscaled.state file in dir.
func writeStateFile(t *testing.T, dir, profileID, fqdn string) {
	t.Helper()

	profiles := map[string]struct {
		Name string `json:"Name"`
	}{
		profileID: {Name: fqdn},
	}
	profilesJSON, err := json.Marshal(profiles)
	if err != nil {
		t.Fatal(err)
	}

	state := map[string]string{
		"_current-profile": base64.StdEncoding.EncodeToString([]byte(profileID)),
		"_profiles":        base64.StdEncoding.EncodeToString(profilesJSON),
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tailscaled.state"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestReadServiceURL_ValidState(t *testing.T) {
	dir := t.TempDir()
	writeStateFile(t, dir, "profile-123", "devagent.happy-llama.ts.net")

	tc := config.TailscaleConfig{Name: "devagent"}
	got, ok := ReadServiceURL(dir, tc)
	if !ok {
		t.Fatal("expected ok=true for valid state file")
	}
	want := "https://devagent.happy-llama.ts.net"
	if got != want {
		t.Errorf("ReadServiceURL() = %q, want %q", got, want)
	}
}

func TestReadServiceURL_KeyFieldLookup(t *testing.T) {
	// Real tailscale state: map key is "4242", but _current-profile decodes to "profile-4242"
	// and the profile has Key: "profile-4242"
	dir := t.TempDir()
	writeStateFileWithKey(t, dir, "profile-4242", "4242", "devagent.example.ts.net")

	tc := config.TailscaleConfig{Name: "devagent"}
	got, ok := ReadServiceURL(dir, tc)
	if !ok {
		t.Fatal("expected ok=true for valid state file with Key field lookup")
	}
	want := "https://devagent.example.ts.net"
	if got != want {
		t.Errorf("ReadServiceURL() = %q, want %q", got, want)
	}
}

func TestReadServiceURL_MissingStateFile(t *testing.T) {
	dir := t.TempDir()

	tc := config.TailscaleConfig{Name: "devagent"}
	got, ok := ReadServiceURL(dir, tc)
	if ok {
		t.Fatal("expected ok=false for missing state file")
	}
	want := "https://devagent.<tailnet>.ts.net"
	if got != want {
		t.Errorf("ReadServiceURL() = %q, want %q", got, want)
	}
}

func TestReadServiceURL_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tailscaled.state"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	tc := config.TailscaleConfig{Name: "devagent"}
	got, ok := ReadServiceURL(dir, tc)
	if ok {
		t.Fatal("expected ok=false for corrupt JSON")
	}
	want := "https://devagent.<tailnet>.ts.net"
	if got != want {
		t.Errorf("ReadServiceURL() = %q, want %q", got, want)
	}
}

func TestReadServiceURL_PlaintextMode(t *testing.T) {
	dir := t.TempDir()
	writeStateFile(t, dir, "profile-123", "devagent.happy-llama.ts.net")

	tc := config.TailscaleConfig{Name: "devagent", Plaintext: true}
	got, ok := ReadServiceURL(dir, tc)
	if !ok {
		t.Fatal("expected ok=true for valid state file")
	}
	want := "http://devagent.happy-llama.ts.net"
	if got != want {
		t.Errorf("ReadServiceURL() = %q, want %q", got, want)
	}
}
