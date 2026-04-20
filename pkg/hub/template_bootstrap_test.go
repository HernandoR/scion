// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !no_sqlite

package hub

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
	"github.com/GoogleCloudPlatform/scion/pkg/store/sqlite"
)

// testTemplateBootstrapServer creates a hub Server backed by an in-memory
// SQLite store and a mock storage, suitable for template bootstrap tests.
func testTemplateBootstrapServer(t *testing.T) (*Server, store.Store, *mockStorage) {
	t.Helper()
	s, err := sqlite.New(":memory:")
	if err != nil {
		if strings.Contains(err.Error(), "sqlite driver not registered") {
			t.Skip("Skipping: sqlite driver not registered")
		}
		t.Fatalf("failed to create test store: %v", err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	cfg := DefaultServerConfig()
	srv, err := New(cfg, s)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	t.Cleanup(func() { srv.Shutdown(context.Background()) })

	stor := newMockStorage("test-bucket")
	srv.SetStorage(stor)

	return srv, s, stor
}

// makeTemplateDir creates a temp directory with template files and returns
// the parent templates directory. The template is created as a subdirectory
// named templateName.
func makeTemplateDir(t *testing.T, templateName string, files map[string]string) string {
	t.Helper()
	templatesDir := t.TempDir()
	templateDir := filepath.Join(templatesDir, templateName)
	for relPath, content := range files {
		full := filepath.Join(templateDir, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return templatesDir
}

func TestBootstrapTemplatesFromDir_ImportsTemplates(t *testing.T) {
	srv, s, stor := testTemplateBootstrapServer(t)
	ctx := context.Background()

	templatesDir := makeTemplateDir(t, "my-template", map[string]string{
		"home/.bashrc": "# bashrc content",
		"README.md":    "hello",
	})

	if err := srv.BootstrapTemplatesFromDir(ctx, templatesDir); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	// Verify a template was created in the store
	result, err := s.ListTemplates(ctx, store.TemplateFilter{}, store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 1 {
		t.Fatalf("expected 1 template, got %d", result.TotalCount)
	}

	tmpl := result.Items[0]
	if tmpl.Name != "my-template" {
		t.Errorf("expected name 'my-template', got %q", tmpl.Name)
	}
	if tmpl.Status != store.TemplateStatusActive {
		t.Errorf("expected status active, got %q", tmpl.Status)
	}
	if tmpl.Scope != store.TemplateScopeGlobal {
		t.Errorf("expected scope global, got %q", tmpl.Scope)
	}
	if len(tmpl.Files) != 2 {
		t.Errorf("expected 2 files in manifest, got %d", len(tmpl.Files))
	}
	if tmpl.ContentHash == "" {
		t.Error("expected non-empty content hash")
	}

	// Verify files were uploaded to storage
	if len(stor.objects) != 2 {
		t.Errorf("expected 2 objects in storage, got %d", len(stor.objects))
	}
}

func TestBootstrapTemplatesFromDir_ImportsNewAlongsideExisting(t *testing.T) {
	srv, s, stor := testTemplateBootstrapServer(t)
	ctx := context.Background()

	// Pre-create a template in the store
	existing := &store.Template{
		ID:     "existing-id",
		Name:   "existing",
		Slug:   "existing",
		Scope:  store.TemplateScopeGlobal,
		Status: store.TemplateStatusActive,
	}
	if err := s.CreateTemplate(ctx, existing); err != nil {
		t.Fatal(err)
	}

	templatesDir := makeTemplateDir(t, "new-template", map[string]string{
		"file.txt": "content",
	})

	if err := srv.BootstrapTemplatesFromDir(ctx, templatesDir); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	// Verify the new template was imported alongside the existing one
	result, err := s.ListTemplates(ctx, store.TemplateFilter{}, store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 2 {
		t.Fatalf("expected 2 templates (existing + new), got %d", result.TotalCount)
	}

	// Verify the new template files were uploaded
	if len(stor.objects) != 1 {
		t.Errorf("expected 1 object in storage (new template file), got %d", len(stor.objects))
	}
}

func TestBootstrapTemplatesFromDir_SyncsChangedTemplate(t *testing.T) {
	srv, s, stor := testTemplateBootstrapServer(t)
	ctx := context.Background()

	// First bootstrap
	templatesDir := makeTemplateDir(t, "my-template", map[string]string{
		"file.txt": "original content",
	})

	if err := srv.BootstrapTemplatesFromDir(ctx, templatesDir); err != nil {
		t.Fatalf("first bootstrap failed: %v", err)
	}

	// Verify initial state
	result, err := s.ListTemplates(ctx, store.TemplateFilter{}, store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 1 {
		t.Fatalf("expected 1 template, got %d", result.TotalCount)
	}
	originalHash := result.Items[0].ContentHash
	_ = stor // storage is used during upload

	// Modify the template file on disk
	if err := os.WriteFile(filepath.Join(templatesDir, "my-template", "file.txt"), []byte("updated content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second bootstrap should detect the change and update
	if err := srv.BootstrapTemplatesFromDir(ctx, templatesDir); err != nil {
		t.Fatalf("second bootstrap failed: %v", err)
	}

	// Verify the template was updated with a new content hash
	result, err = s.ListTemplates(ctx, store.TemplateFilter{}, store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 1 {
		t.Fatalf("expected 1 template, got %d", result.TotalCount)
	}
	if result.Items[0].ContentHash == originalHash {
		t.Error("expected content hash to change after file update")
	}
}

func TestBootstrapTemplatesFromDir_SkipsUnchangedTemplate(t *testing.T) {
	srv, s, stor := testTemplateBootstrapServer(t)
	ctx := context.Background()

	templatesDir := makeTemplateDir(t, "my-template", map[string]string{
		"file.txt": "stable content",
	})

	if err := srv.BootstrapTemplatesFromDir(ctx, templatesDir); err != nil {
		t.Fatalf("first bootstrap failed: %v", err)
	}

	result, _ := s.ListTemplates(ctx, store.TemplateFilter{}, store.ListOptions{Limit: 10})
	originalHash := result.Items[0].ContentHash
	uploadCountAfterFirst := len(stor.objects)

	// Second bootstrap with no changes
	if err := srv.BootstrapTemplatesFromDir(ctx, templatesDir); err != nil {
		t.Fatalf("second bootstrap failed: %v", err)
	}

	result, _ = s.ListTemplates(ctx, store.TemplateFilter{}, store.ListOptions{Limit: 10})
	if result.Items[0].ContentHash != originalHash {
		t.Error("content hash should not change when files are unchanged")
	}
	if len(stor.objects) != uploadCountAfterFirst {
		t.Errorf("expected no new uploads, storage objects went from %d to %d",
			uploadCountAfterFirst, len(stor.objects))
	}
}

func TestBootstrapTemplatesFromDir_NoopWhenNoStorage(t *testing.T) {
	// Create server without storage
	s, err := sqlite.New(":memory:")
	if err != nil {
		if strings.Contains(err.Error(), "sqlite driver not registered") {
			t.Skip("Skipping: sqlite driver not registered")
		}
		t.Fatalf("failed to create test store: %v", err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	cfg := DefaultServerConfig()
	srv, err := New(cfg, s)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	t.Cleanup(func() { srv.Shutdown(context.Background()) })
	// Deliberately not calling srv.SetStorage()

	ctx := context.Background()
	templatesDir := makeTemplateDir(t, "some-template", map[string]string{
		"file.txt": "content",
	})

	// Should not error, just skip
	if err := srv.BootstrapTemplatesFromDir(ctx, templatesDir); err != nil {
		t.Fatalf("bootstrap should not fail without storage: %v", err)
	}

	// Verify no templates were created
	result, err := s.ListTemplates(ctx, store.TemplateFilter{}, store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 0 {
		t.Fatalf("expected 0 templates, got %d", result.TotalCount)
	}
}

func TestBootstrapTemplatesFromDir_EmptyDirectory(t *testing.T) {
	srv, s, _ := testTemplateBootstrapServer(t)
	ctx := context.Background()

	// Create an empty templates directory
	templatesDir := t.TempDir()

	if err := srv.BootstrapTemplatesFromDir(ctx, templatesDir); err != nil {
		t.Fatalf("bootstrap failed on empty dir: %v", err)
	}

	result, err := s.ListTemplates(ctx, store.TemplateFilter{}, store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 0 {
		t.Fatalf("expected 0 templates, got %d", result.TotalCount)
	}
}

func TestBootstrapTemplatesFromDir_NonexistentDirectory(t *testing.T) {
	srv, _, _ := testTemplateBootstrapServer(t)
	ctx := context.Background()

	if err := srv.BootstrapTemplatesFromDir(ctx, "/nonexistent/path"); err != nil {
		t.Fatalf("bootstrap should silently skip nonexistent directory: %v", err)
	}
}

func TestBootstrapTemplatesFromDir_MultipleTemplates(t *testing.T) {
	srv, s, _ := testTemplateBootstrapServer(t)
	ctx := context.Background()

	templatesDir := t.TempDir()

	// Create two template subdirectories
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(templatesDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.txt"), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := srv.BootstrapTemplatesFromDir(ctx, templatesDir); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	result, err := s.ListTemplates(ctx, store.TemplateFilter{}, store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 2 {
		t.Fatalf("expected 2 templates, got %d", result.TotalCount)
	}
}

func TestBootstrapTemplatesFromDir_SkipsNonDirectories(t *testing.T) {
	srv, s, _ := testTemplateBootstrapServer(t)
	ctx := context.Background()

	templatesDir := t.TempDir()

	// Create a regular file (not a directory) at the top level
	if err := os.WriteFile(filepath.Join(templatesDir, "not-a-template.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create one valid template
	dir := filepath.Join(templatesDir, "valid")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := srv.BootstrapTemplatesFromDir(ctx, templatesDir); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	result, err := s.ListTemplates(ctx, store.TemplateFilter{}, store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 1 {
		t.Fatalf("expected 1 template (skipping file), got %d", result.TotalCount)
	}
}

// TestSyncExistingTemplate_ForceReconcilesStorage verifies that a forced
// re-sync re-uploads modified files, uploads added files, and deletes files
// that are no longer present on disk. This mirrors the import-from-URL path
// where the user expects re-import to fully reflect their source changes.
func TestSyncExistingTemplate_ForceReconcilesStorage(t *testing.T) {
	srv, s, stor := testTemplateBootstrapServer(t)
	ctx := context.Background()

	// Initial bootstrap of a template with three files.
	templatesDir := makeTemplateDir(t, "my-template", map[string]string{
		"file-keep.txt":   "keep original",
		"file-update.txt": "before",
		"file-remove.txt": "stale",
	})
	templateDir := filepath.Join(templatesDir, "my-template")

	if err := srv.bootstrapSingleTemplate(ctx, "my-template", templateDir, store.TemplateScopeGlobal, ""); err != nil {
		t.Fatalf("initial bootstrap failed: %v", err)
	}

	existing, err := s.GetTemplateBySlug(ctx, "my-template", store.TemplateScopeGlobal, "")
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	originalHash := existing.ContentHash
	if len(stor.objects) != 3 {
		t.Fatalf("expected 3 storage objects after bootstrap, got %d", len(stor.objects))
	}

	// Modify the source: update one file, delete one, add a new one.
	if err := os.WriteFile(filepath.Join(templateDir, "file-update.txt"), []byte("after"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(templateDir, "file-remove.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "file-new.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	changed, err := srv.syncExistingTemplate(ctx, existing, templateDir, true)
	if err != nil {
		t.Fatalf("syncExistingTemplate failed: %v", err)
	}
	if !changed {
		t.Error("expected changed=true when content differs")
	}

	// DB manifest reflects the new file set.
	got, err := s.GetTemplateBySlug(ctx, "my-template", store.TemplateScopeGlobal, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentHash == originalHash {
		t.Error("expected ContentHash to change after reconcile")
	}
	wantPaths := map[string]bool{"file-keep.txt": true, "file-update.txt": true, "file-new.txt": true}
	if len(got.Files) != len(wantPaths) {
		t.Errorf("expected %d files in manifest, got %d", len(wantPaths), len(got.Files))
	}
	for _, f := range got.Files {
		if !wantPaths[f.Path] {
			t.Errorf("unexpected file in manifest: %q", f.Path)
		}
	}

	// Storage reflects the new set: removed file is gone, new file is present.
	storagePath := got.StoragePath
	if _, exists := stor.objects[storagePath+"/file-remove.txt"]; exists {
		t.Error("expected file-remove.txt to be deleted from storage")
	}
	if _, exists := stor.objects[storagePath+"/file-new.txt"]; !exists {
		t.Error("expected file-new.txt to be uploaded to storage")
	}
	if _, exists := stor.objects[storagePath+"/file-update.txt"]; !exists {
		t.Error("expected file-update.txt to remain in storage after re-upload")
	}
	if len(stor.objects) != 3 {
		t.Errorf("expected 3 storage objects after reconcile, got %d", len(stor.objects))
	}
}

// TestSyncExistingTemplate_ForceWithoutChangesStillReuploads verifies that
// force=true re-uploads even when the source files are unchanged, so that
// the storage state is brought back into sync with the manifest if a user
// has reason to suspect drift.
func TestSyncExistingTemplate_ForceWithoutChangesStillReuploads(t *testing.T) {
	srv, s, stor := testTemplateBootstrapServer(t)
	ctx := context.Background()

	templatesDir := makeTemplateDir(t, "stable", map[string]string{
		"only.txt": "same content",
	})
	templateDir := filepath.Join(templatesDir, "stable")

	if err := srv.bootstrapSingleTemplate(ctx, "stable", templateDir, store.TemplateScopeGlobal, ""); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	existing, err := s.GetTemplateBySlug(ctx, "stable", store.TemplateScopeGlobal, "")
	if err != nil {
		t.Fatal(err)
	}

	// Manually drop the storage object to simulate drift.
	storagePath := existing.StoragePath
	delete(stor.objects, storagePath+"/only.txt")

	if _, err := srv.syncExistingTemplate(ctx, existing, templateDir, true); err != nil {
		t.Fatalf("syncExistingTemplate failed: %v", err)
	}

	if _, exists := stor.objects[storagePath+"/only.txt"]; !exists {
		t.Error("expected only.txt to be re-uploaded by forced sync")
	}
}

// TestSyncExistingTemplate_PopulatesNewHashForLaterAgents verifies that after
// a forced re-sync, a freshly resolved template (the path used when creating a
// new agent) carries the updated ContentHash. This is the chain that ensures
// new agents created after a re-import use the new template version.
func TestSyncExistingTemplate_PopulatesNewHashForLaterAgents(t *testing.T) {
	srv, s, _ := testTemplateBootstrapServer(t)
	ctx := context.Background()

	templatesDir := makeTemplateDir(t, "claude-template", map[string]string{
		"home/.bashrc": "# v1",
	})
	templateDir := filepath.Join(templatesDir, "claude-template")

	if err := srv.bootstrapSingleTemplate(ctx, "claude-template", templateDir, store.TemplateScopeGlobal, ""); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	v1, err := srv.resolveTemplate(ctx, "claude-template", "")
	if err != nil || v1 == nil {
		t.Fatalf("resolveTemplate v1: %v", err)
	}
	v1Hash := v1.ContentHash
	if v1Hash == "" {
		t.Fatal("expected non-empty hash after bootstrap")
	}

	// Edit the source as the user would after editing the git repo.
	if err := os.WriteFile(filepath.Join(templateDir, "home/.bashrc"), []byte("# v2"), 0644); err != nil {
		t.Fatal(err)
	}

	existing, err := s.GetTemplateBySlug(ctx, "claude-template", store.TemplateScopeGlobal, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.syncExistingTemplate(ctx, existing, templateDir, true); err != nil {
		t.Fatalf("sync (force) failed: %v", err)
	}

	v2, err := srv.resolveTemplate(ctx, "claude-template", "")
	if err != nil || v2 == nil {
		t.Fatalf("resolveTemplate v2: %v", err)
	}
	if v2.ContentHash == v1Hash {
		t.Errorf("expected ContentHash to change after re-sync; v1=%s v2=%s", v1Hash, v2.ContentHash)
	}
}

func TestDetectHarnessFromConfig_NameBased(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"claude", "claude"},
		{"my-claude-template", "claude"},
		{"gemini", "gemini"},
		{"custom-gemini-pro", "gemini"},
		{"opencode", "opencode"},
		{"codex", "codex"},
		{"default", ""},
		{"my-custom", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use an empty temp dir so config loading returns empty config
			dir := t.TempDir()
			got := detectHarnessFromConfig(dir, tt.name)
			if got != tt.expected {
				t.Errorf("detectHarnessFromConfig(%q, %q) = %q, want %q", dir, tt.name, got, tt.expected)
			}
		})
	}
}

func TestDetectHarnessFromConfig_FromConfigFile(t *testing.T) {
	dir := t.TempDir()

	// Write a scion-agent.yaml with a harness_config field
	configContent := `harness_config: claude
`
	if err := os.WriteFile(filepath.Join(dir, "scion-agent.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	got := detectHarnessFromConfig(dir, "my-template")
	if got != "claude" {
		t.Errorf("expected 'claude' from config, got %q", got)
	}
}

func TestDetectHarnessFromConfig_DefaultHarnessConfig(t *testing.T) {
	dir := t.TempDir()

	configContent := `default_harness_config: gemini
`
	if err := os.WriteFile(filepath.Join(dir, "scion-agent.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	got := detectHarnessFromConfig(dir, "my-template")
	if got != "gemini" {
		t.Errorf("expected 'gemini' from config, got %q", got)
	}
}

func TestDetectHarnessFromConfig_HarnessField(t *testing.T) {
	dir := t.TempDir()

	configContent := `harness: codex
`
	if err := os.WriteFile(filepath.Join(dir, "scion-agent.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	got := detectHarnessFromConfig(dir, "my-template")
	if got != "codex" {
		t.Errorf("expected 'codex' from config, got %q", got)
	}
}
