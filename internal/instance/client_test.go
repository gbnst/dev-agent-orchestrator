package instance

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
