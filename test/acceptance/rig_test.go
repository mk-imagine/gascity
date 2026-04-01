//go:build acceptance_a

// Rig management acceptance tests.
//
// These exercise gc rig add, list, suspend, resume, status, and restart
// as a black box. Rigs are how users register external projects into a
// city for multi-project orchestration.
package acceptance_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	helpers "github.com/gastownhall/gascity/test/acceptance/helpers"
)

// --- gc rig (bare command) ---

func TestRig_NoSubcommand_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig")
	if err == nil {
		t.Fatal("expected error for bare 'gc rig', got success")
	}
	if !strings.Contains(out, "missing subcommand") {
		t.Errorf("expected 'missing subcommand' message, got:\n%s", out)
	}
}

func TestRig_UnknownSubcommand_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig", "bogus")
	if err == nil {
		t.Fatal("expected error for 'gc rig bogus', got success")
	}
	if !strings.Contains(out, "unknown subcommand") {
		t.Errorf("expected 'unknown subcommand' message, got:\n%s", out)
	}
}

// --- gc rig add ---

func TestRigAdd_ValidPath_Succeeds(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	rigDir := makeGitRepo(t)
	out, err := c.GC("rig", "add", rigDir)
	if err != nil {
		t.Fatalf("gc rig add failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Rig added") {
		t.Errorf("expected 'Rig added' in output, got:\n%s", out)
	}
}

func TestRigAdd_MissingPath_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig", "add")
	if err == nil {
		t.Fatal("expected error for 'gc rig add' without path, got success")
	}
	if !strings.Contains(out, "missing path") {
		t.Errorf("expected 'missing path' message, got:\n%s", out)
	}
}

func TestRigAdd_NotADirectory_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	tmpFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := c.GC("rig", "add", tmpFile)
	if err == nil {
		t.Fatal("expected error for file path, got success")
	}
	if !strings.Contains(out, "not a directory") {
		t.Errorf("expected 'not a directory' message, got:\n%s", out)
	}
}

func TestRigAdd_DuplicateName_DifferentPath_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	rig1 := makeGitRepo(t)
	if _, err := c.GC("rig", "add", rig1); err != nil {
		t.Fatalf("first rig add failed: %v", err)
	}

	// Create a second repo with the same directory name in a different parent.
	rigName := filepath.Base(rig1)
	rig2Parent := t.TempDir()
	rig2 := filepath.Join(rig2Parent, rigName)
	initGitRepo(t, rig2)

	out, err := c.GC("rig", "add", rig2)
	if err == nil {
		t.Fatal("expected error for duplicate rig name with different path, got success")
	}
	if !strings.Contains(out, "already registered") {
		t.Errorf("expected 'already registered' message, got:\n%s", out)
	}
}

func TestRigAdd_StartSuspended_AddsSuspended(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	rigDir := makeGitRepo(t)
	out, err := c.GC("rig", "add", "--start-suspended", rigDir)
	if err != nil {
		t.Fatalf("gc rig add --start-suspended failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "suspended") {
		t.Errorf("expected 'suspended' in output, got:\n%s", out)
	}

	// Verify rig list shows it as suspended.
	listOut, err := c.GC("rig", "list")
	if err != nil {
		t.Fatalf("gc rig list failed: %v\n%s", err, listOut)
	}
	if !strings.Contains(listOut, "suspended") {
		t.Errorf("expected 'suspended' in rig list output, got:\n%s", listOut)
	}
}

// --- gc rig list ---

func TestRigList_FreshCity_ShowsHQ(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig", "list")
	if err != nil {
		t.Fatalf("gc rig list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "HQ") {
		t.Errorf("expected 'HQ' in rig list output, got:\n%s", out)
	}
}

func TestRigList_AfterAdd_ShowsRig(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	rigDir := makeGitRepo(t)
	rigName := filepath.Base(rigDir)
	if _, err := c.GC("rig", "add", rigDir); err != nil {
		t.Fatalf("gc rig add failed: %v", err)
	}

	out, err := c.GC("rig", "list")
	if err != nil {
		t.Fatalf("gc rig list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, rigName) {
		t.Errorf("expected rig name %q in list output, got:\n%s", rigName, out)
	}
}

// --- gc rig suspend ---

func TestRigSuspend_ExistingRig_Succeeds(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	rigDir := makeGitRepo(t)
	rigName := filepath.Base(rigDir)
	if _, err := c.GC("rig", "add", rigDir); err != nil {
		t.Fatalf("gc rig add failed: %v", err)
	}

	out, err := c.GC("rig", "suspend", rigName)
	if err != nil {
		t.Fatalf("gc rig suspend failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Suspended rig") {
		t.Errorf("expected 'Suspended rig' in output, got:\n%s", out)
	}
}

func TestRigSuspend_NonexistentRig_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig", "suspend", "no-such-rig")
	if err == nil {
		t.Fatal("expected error for nonexistent rig, got success")
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' message, got:\n%s", out)
	}
}

func TestRigSuspend_MissingName_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig", "suspend")
	if err == nil {
		t.Fatal("expected error for missing rig name, got success")
	}
	if !strings.Contains(out, "missing rig name") {
		t.Errorf("expected 'missing rig name' message, got:\n%s", out)
	}
}

// --- gc rig resume ---

func TestRigResume_AfterSuspend_Succeeds(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	rigDir := makeGitRepo(t)
	rigName := filepath.Base(rigDir)
	if _, err := c.GC("rig", "add", rigDir); err != nil {
		t.Fatalf("gc rig add failed: %v", err)
	}
	if _, err := c.GC("rig", "suspend", rigName); err != nil {
		t.Fatalf("gc rig suspend failed: %v", err)
	}

	out, err := c.GC("rig", "resume", rigName)
	if err != nil {
		t.Fatalf("gc rig resume failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Resumed rig") {
		t.Errorf("expected 'Resumed rig' in output, got:\n%s", out)
	}
}

func TestRigResume_NonexistentRig_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig", "resume", "no-such-rig")
	if err == nil {
		t.Fatal("expected error for nonexistent rig, got success")
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' message, got:\n%s", out)
	}
}

func TestRigResume_MissingName_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig", "resume")
	if err == nil {
		t.Fatal("expected error for missing rig name, got success")
	}
	if !strings.Contains(out, "missing rig name") {
		t.Errorf("expected 'missing rig name' message, got:\n%s", out)
	}
}

// --- gc rig status ---

func TestRigStatus_ExistingRig_ShowsDetails(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	rigDir := makeGitRepo(t)
	rigName := filepath.Base(rigDir)
	if _, err := c.GC("rig", "add", rigDir); err != nil {
		t.Fatalf("gc rig add failed: %v", err)
	}

	out, err := c.GC("rig", "status", rigName)
	if err != nil {
		t.Fatalf("gc rig status failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, rigName) {
		t.Errorf("expected rig name in status output, got:\n%s", out)
	}
	if !strings.Contains(out, "Path:") {
		t.Errorf("expected 'Path:' in status output, got:\n%s", out)
	}
}

func TestRigStatus_NonexistentRig_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig", "status", "no-such-rig")
	if err == nil {
		t.Fatal("expected error for nonexistent rig, got success")
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' message, got:\n%s", out)
	}
}

func TestRigStatus_MissingName_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig", "status")
	if err == nil {
		t.Fatal("expected error for missing rig name, got success")
	}
	if !strings.Contains(out, "missing rig name") {
		t.Errorf("expected 'missing rig name' message, got:\n%s", out)
	}
}

// --- gc rig restart ---

func TestRigRestart_ExistingRig_Succeeds(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	rigDir := makeGitRepo(t)
	rigName := filepath.Base(rigDir)
	if _, err := c.GC("rig", "add", rigDir); err != nil {
		t.Fatalf("gc rig add failed: %v", err)
	}

	out, err := c.GC("rig", "restart", rigName)
	if err != nil {
		t.Fatalf("gc rig restart failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Restarted") {
		t.Errorf("expected 'Restarted' in output, got:\n%s", out)
	}
}

func TestRigRestart_NonexistentRig_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig", "restart", "no-such-rig")
	if err == nil {
		t.Fatal("expected error for nonexistent rig, got success")
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' message, got:\n%s", out)
	}
}

func TestRigRestart_MissingName_ReturnsError(t *testing.T) {
	c := helpers.NewCity(t, testEnv)
	c.Init("claude")

	out, err := c.GC("rig", "restart")
	if err == nil {
		t.Fatal("expected error for missing rig name, got success")
	}
	if !strings.Contains(out, "missing rig name") {
		t.Errorf("expected 'missing rig name' message, got:\n%s", out)
	}
}

// --- helpers ---

// makeGitRepo creates a minimal git repo in a temp directory and returns its path.
func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "test-rig")
	initGitRepo(t, dir)
	return dir
}

// initGitRepo creates a minimal git repo at the given path.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("creating git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("writing HEAD: %v", err)
	}
}
