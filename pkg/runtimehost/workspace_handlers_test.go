package runtimehost

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ptone/scion-agent/pkg/agent"
	"github.com/ptone/scion-agent/pkg/api"
	"github.com/ptone/scion-agent/pkg/runtime"
)

// mockAgentManager implements agent.Manager for testing workspace handlers.
type mockAgentManager struct {
	agents []api.AgentInfo
}

func (m *mockAgentManager) Provision(ctx context.Context, opts api.StartOptions) (*api.ScionConfig, error) {
	return nil, nil
}

func (m *mockAgentManager) Start(ctx context.Context, opts api.StartOptions) (*api.AgentInfo, error) {
	return nil, nil
}

func (m *mockAgentManager) Stop(ctx context.Context, name string) error {
	return nil
}

func (m *mockAgentManager) Delete(ctx context.Context, name string, deleteFiles bool, grovePath string, removeBranch bool) (bool, error) {
	return true, nil
}

func (m *mockAgentManager) List(ctx context.Context, filter map[string]string) ([]api.AgentInfo, error) {
	return m.agents, nil
}

func (m *mockAgentManager) Message(ctx context.Context, name, message string, interrupt bool) error {
	return nil
}

func (m *mockAgentManager) Watch(ctx context.Context, name string) (<-chan api.StatusEvent, error) {
	return nil, nil
}

// Ensure mockAgentManager implements agent.Manager
var _ agent.Manager = (*mockAgentManager)(nil)

func TestWorkspaceUploadValidation(t *testing.T) {
	cfg := DefaultServerConfig()
	mgr := &mockAgentManager{}
	rt := &runtime.MockRuntime{}
	srv := New(cfg, mgr, rt)

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing agentId",
			body:       WorkspaceUploadRequest{StoragePath: "workspaces/g/a"},
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidationError,
		},
		{
			name:       "missing storagePath",
			body:       WorkspaceUploadRequest{AgentID: "test-agent"},
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidationError,
		},
		{
			name:       "missing bucket when not configured",
			body:       WorkspaceUploadRequest{AgentID: "test-agent", StoragePath: "workspaces/g/a"},
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidationError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace/upload", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			srv.handleWorkspaceUpload(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}

			var errResp ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if errResp.Error.Code != tt.wantCode {
				t.Errorf("got error code %s, want %s", errResp.Error.Code, tt.wantCode)
			}
		})
	}
}

func TestWorkspaceApplyValidation(t *testing.T) {
	cfg := DefaultServerConfig()
	mgr := &mockAgentManager{}
	rt := &runtime.MockRuntime{}
	srv := New(cfg, mgr, rt)

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing agentId",
			body:       WorkspaceApplyRequest{StoragePath: "workspaces/g/a"},
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidationError,
		},
		{
			name:       "missing storagePath",
			body:       WorkspaceApplyRequest{AgentID: "test-agent"},
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidationError,
		},
		{
			name:       "missing bucket when not configured",
			body:       WorkspaceApplyRequest{AgentID: "test-agent", StoragePath: "workspaces/g/a"},
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeValidationError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace/apply", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			srv.handleWorkspaceApply(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}

			var errResp ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if errResp.Error.Code != tt.wantCode {
				t.Errorf("got error code %s, want %s", errResp.Error.Code, tt.wantCode)
			}
		})
	}
}

func TestWorkspaceUploadMethodNotAllowed(t *testing.T) {
	cfg := DefaultServerConfig()
	mgr := &mockAgentManager{}
	rt := &runtime.MockRuntime{}
	srv := New(cfg, mgr, rt)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/workspace/upload", nil)
			rec := httptest.NewRecorder()

			srv.handleWorkspaceUpload(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("got status %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestWorkspaceApplyMethodNotAllowed(t *testing.T) {
	cfg := DefaultServerConfig()
	mgr := &mockAgentManager{}
	rt := &runtime.MockRuntime{}
	srv := New(cfg, mgr, rt)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/workspace/apply", nil)
			rec := httptest.NewRecorder()

			srv.handleWorkspaceApply(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("got status %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestWorkspaceUploadAgentNotFound(t *testing.T) {
	cfg := DefaultServerConfig()
	cfg.StorageBucket = "test-bucket"
	mgr := &mockAgentManager{agents: []api.AgentInfo{}} // No agents
	rt := &runtime.MockRuntime{}
	srv := New(cfg, mgr, rt)

	body := WorkspaceUploadRequest{
		AgentID:     "nonexistent-agent",
		StoragePath: "workspaces/grove/agent",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace/upload", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.handleWorkspaceUpload(rec, req)

	// Agent not found should result in a runtime error (since we can't find the workspace path)
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want error status", rec.Code)
	}
}

func TestWorkspaceApplyAgentNotFound(t *testing.T) {
	cfg := DefaultServerConfig()
	cfg.StorageBucket = "test-bucket"
	mgr := &mockAgentManager{agents: []api.AgentInfo{}} // No agents
	rt := &runtime.MockRuntime{}
	srv := New(cfg, mgr, rt)

	body := WorkspaceApplyRequest{
		AgentID:     "nonexistent-agent",
		StoragePath: "workspaces/grove/agent",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace/apply", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.handleWorkspaceApply(rec, req)

	// Agent not found should result in a runtime error (since we can't find the workspace path)
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want error status", rec.Code)
	}
}

func TestBuildWorkspaceManifest(t *testing.T) {
	cfg := DefaultServerConfig()
	mgr := &mockAgentManager{}
	rt := &runtime.MockRuntime{}
	srv := New(cfg, mgr, rt)

	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "workspace-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := map[string]string{
		"file1.txt":        "content1",
		"subdir/file2.txt": "content2",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	// Build manifest
	manifest, err := srv.buildWorkspaceManifest(tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to build manifest: %v", err)
	}

	// Verify manifest
	if manifest.Version != "1.0" {
		t.Errorf("got version %s, want 1.0", manifest.Version)
	}

	if len(manifest.Files) != 2 {
		t.Errorf("got %d files, want 2", len(manifest.Files))
	}

	// Check that files are present
	fileMap := make(map[string]bool)
	for _, f := range manifest.Files {
		fileMap[f.Path] = true
	}

	for path := range testFiles {
		if !fileMap[path] {
			t.Errorf("missing file in manifest: %s", path)
		}
	}
}

func TestBuildWorkspaceManifestWithExcludes(t *testing.T) {
	cfg := DefaultServerConfig()
	mgr := &mockAgentManager{}
	rt := &runtime.MockRuntime{}
	srv := New(cfg, mgr, rt)

	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "workspace-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files including some that should be excluded
	testFiles := map[string]string{
		"file1.txt":               "content1",
		"node_modules/package.js": "content2",
		".git/config":             "content3",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	// Build manifest with default excludes
	manifest, err := srv.buildWorkspaceManifest(tmpDir, []string{"node_modules/**"})
	if err != nil {
		t.Fatalf("failed to build manifest: %v", err)
	}

	// Should only have file1.txt (git and node_modules excluded)
	if len(manifest.Files) != 1 {
		t.Errorf("got %d files, want 1 (expected excludes to work)", len(manifest.Files))
	}

	if len(manifest.Files) > 0 && manifest.Files[0].Path != "file1.txt" {
		t.Errorf("got file %s, want file1.txt", manifest.Files[0].Path)
	}
}

func TestCountWorkspaceFiles(t *testing.T) {
	cfg := DefaultServerConfig()
	mgr := &mockAgentManager{}
	rt := &runtime.MockRuntime{}
	srv := New(cfg, mgr, rt)

	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "workspace-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := map[string]string{
		"file1.txt":        "content1",
		"subdir/file2.txt": "content2content2",
	}

	var expectedSize int64
	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		expectedSize += int64(len(content))
	}

	count, size := srv.countWorkspaceFiles(tmpDir)

	if count != 2 {
		t.Errorf("got count %d, want 2", count)
	}

	if size != expectedSize {
		t.Errorf("got size %d, want %d", size, expectedSize)
	}
}

func TestWorkspaceRoutesRegistered(t *testing.T) {
	cfg := DefaultServerConfig()
	mgr := &mockAgentManager{}
	rt := &runtime.MockRuntime{}
	srv := New(cfg, mgr, rt)

	handler := srv.Handler()

	// Test that workspace routes are registered
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/workspace/upload"},
		{http.MethodPost, "/api/v1/workspace/apply"},
	}

	for _, route := range routes {
		t.Run(route.path, func(t *testing.T) {
			// Make a request without body to verify route exists
			req := httptest.NewRequest(route.method, route.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// Should get bad request (missing body), not 404
			if rec.Code == http.StatusNotFound {
				t.Errorf("route %s %s not registered", route.method, route.path)
			}
		})
	}
}
