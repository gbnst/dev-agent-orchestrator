package instance

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_List(t *testing.T) {
	// Mock server that returns project JSON
	want := `{"projects":[],"unmatched":[]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(want))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("List() = %q, want %q", string(got), want)
	}
}

func TestClient_List_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.List()
	if err == nil {
		t.Fatal("List() should fail on server error")
	}
}

// Helper method tests

func TestClient_Post_Success(t *testing.T) {
	want := `{"status":"started"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/test" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(want))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.post("/api/test")
	if err != nil {
		t.Fatalf("post() error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("post() = %q, want %q", string(got), want)
	}
}

func TestClient_Post_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.post("/api/test")
	if err == nil {
		t.Fatal("post() should fail on error response")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("bad request")) {
		t.Fatalf("Error should contain 'bad request', got: %v", err)
	}
}

func TestClient_Delete_Success(t *testing.T) {
	want := `{"status":"destroyed"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/test" && r.Method == "DELETE" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(want))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.delete("/api/test")
	if err != nil {
		t.Fatalf("delete() error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("delete() = %q, want %q", string(got), want)
	}
}

func TestClient_PostJSON_SendsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/test" && r.Method == "POST" {
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if req["name"] != "dev" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"name":"dev"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.postJSON("/api/test", map[string]string{"name": "dev"})
	if err != nil {
		t.Fatalf("postJSON() error: %v", err)
	}
	if string(got) != `{"name":"dev"}` {
		t.Fatalf("postJSON() = %q, want %q", string(got), `{"name":"dev"}`)
	}
}

func TestClient_PostJSON_SetsContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Content-Type not application/json"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.postJSON("/api/test", map[string]string{})
	if err != nil {
		t.Fatalf("postJSON() error: %v", err)
	}
}

// Typed method tests

func TestClient_StartContainer_CallsCorrectEndpoint(t *testing.T) {
	want := `{"status":"started"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/containers/abc123/start" && r.Method == "POST" {
			w.Write([]byte(want))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.StartContainer("abc123")
	if err != nil {
		t.Fatalf("StartContainer() error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("StartContainer() = %q, want %q", string(got), want)
	}
}

func TestClient_StopContainer_CallsCorrectEndpoint(t *testing.T) {
	want := `{"status":"stopped"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/containers/abc123/stop" && r.Method == "POST" {
			w.Write([]byte(want))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.StopContainer("abc123")
	if err != nil {
		t.Fatalf("StopContainer() error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("StopContainer() = %q, want %q", string(got), want)
	}
}

func TestClient_DestroyContainer_CallsCorrectEndpoint(t *testing.T) {
	want := `{"status":"destroyed"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/containers/abc123" && r.Method == "DELETE" {
			w.Write([]byte(want))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.DestroyContainer("abc123")
	if err != nil {
		t.Fatalf("DestroyContainer() error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("DestroyContainer() = %q, want %q", string(got), want)
	}
}

func TestClient_CreateSession_CallsCorrectEndpoint(t *testing.T) {
	want := `{"name":"dev"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/containers/abc123/sessions" && r.Method == "POST" {
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req["name"] != "dev" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(want))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.CreateSession("abc123", "dev")
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("CreateSession() = %q, want %q", string(got), want)
	}
}

func TestClient_DestroySession_CallsCorrectEndpoint(t *testing.T) {
	want := `{"status":"destroyed"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/containers/abc123/sessions/dev" && r.Method == "DELETE" {
			w.Write([]byte(want))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.DestroySession("abc123", "dev")
	if err != nil {
		t.Fatalf("DestroySession() error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("DestroySession() = %q, want %q", string(got), want)
	}
}

func TestClient_CreateWorktree_EncodesPath(t *testing.T) {
	projectPath := "/home/user/myproject"
	encoded := base64.URLEncoding.EncodeToString([]byte(projectPath))
	expectedPath := "/api/projects/" + encoded + "/worktrees"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == expectedPath && r.Method == "POST" {
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req["name"] != "feature" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"name":"feature"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.CreateWorktree(projectPath, "feature", false)
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}
	if string(got) != `{"name":"feature"}` {
		t.Fatalf("CreateWorktree() = %q, want %q", string(got), `{"name":"feature"}`)
	}
}

func TestClient_CreateWorktree_NoStart(t *testing.T) {
	projectPath := "/home/user/myproject"
	encoded := base64.URLEncoding.EncodeToString([]byte(projectPath))
	expectedPath := "/api/projects/" + encoded + "/worktrees"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == expectedPath && r.Method == "POST" {
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// Check that no_start is in the request
			if noStart, ok := req["no_start"].(bool); !ok || !noStart {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"no_start should be true"}`))
				return
			}
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"name":"feature"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.CreateWorktree(projectPath, "feature", true)
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}
	if string(got) != `{"name":"feature"}` {
		t.Fatalf("CreateWorktree() = %q, want %q", string(got), `{"name":"feature"}`)
	}
}

func TestClient_NewClientWithTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Create client with 120s timeout
	client := NewClientWithTimeout(srv.URL, 120*time.Second)
	_, err := client.post("/api/test")
	if err != nil {
		t.Fatalf("NewClientWithTimeout() client error: %v", err)
	}
	// Verify timeout is set correctly on httpClient
	if client.httpClient.Timeout.Seconds() != 120.0 {
		t.Fatalf("Timeout not set correctly, got: %v", client.httpClient.Timeout)
	}
}

// Error case tests

func TestClient_StartContainer_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"container not found"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.StartContainer("notfound")
	if err == nil {
		t.Fatal("StartContainer() should fail on 404")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("container not found")) {
		t.Fatalf("Error should contain 'container not found', got: %v", err)
	}
}

func TestClient_StartContainer_AlreadyRunning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"container is already running"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.StartContainer("abc123")
	if err == nil {
		t.Fatal("StartContainer() should fail when already running")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("already running")) {
		t.Fatalf("Error should contain 'already running', got: %v", err)
	}
}

func TestClient_CreateSession_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"session already exists"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.CreateSession("abc123", "dev")
	if err == nil {
		t.Fatal("CreateSession() should fail on conflict")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("already exists")) {
		t.Fatalf("Error should contain 'already exists', got: %v", err)
	}
}

// ReadSession and SendToSession tests

func TestClient_ReadSession_DefaultLines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/containers/abc123/sessions/dev/capture" && r.Method == "GET" {
			if r.URL.RawQuery != "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"unexpected query params"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"content":"line1\nline2","cursor_y":5,"lines_requested":-1}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.ReadSession("abc123", "dev", 0)
	if err != nil {
		t.Fatalf("ReadSession() error: %v", err)
	}
	if string(got) != `{"content":"line1\nline2","cursor_y":5,"lines_requested":-1}` {
		t.Fatalf("ReadSession() = %q", string(got))
	}
}

func TestClient_ReadSession_WithLines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/containers/abc123/sessions/dev/capture" && r.Method == "GET" {
			if r.URL.Query().Get("lines") != "50" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"lines param not 50"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"content":"last 50 lines","cursor_y":50,"lines_requested":50}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.ReadSession("abc123", "dev", 50)
	if err != nil {
		t.Fatalf("ReadSession() error: %v", err)
	}
	if string(got) != `{"content":"last 50 lines","cursor_y":50,"lines_requested":50}` {
		t.Fatalf("ReadSession() = %q", string(got))
	}
}

func TestClient_ReadSessionFromCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/containers/abc123/sessions/dev/capture" && r.Method == "GET" {
			if r.URL.Query().Get("from_cursor") != "12" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"from_cursor param not 12"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"content":"lines from 12","cursor_y":15,"lines_requested":12}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	got, err := client.ReadSessionFromCursor("abc123", "dev", 12)
	if err != nil {
		t.Fatalf("ReadSessionFromCursor() error: %v", err)
	}
	if string(got) != `{"content":"lines from 12","cursor_y":15,"lines_requested":12}` {
		t.Fatalf("ReadSessionFromCursor() = %q", string(got))
	}
}

func TestClient_SendToSession_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/containers/abc123/sessions/dev/send" && r.Method == "POST" {
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if req["text"] != "ls -la" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"text param incorrect"}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	err := client.SendToSession("abc123", "dev", "ls -la")
	if err != nil {
		t.Fatalf("SendToSession() error: %v", err)
	}
}
