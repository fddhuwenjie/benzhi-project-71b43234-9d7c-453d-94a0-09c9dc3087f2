package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

func TestWorkbenchAndJSONValidation(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewServer(application.NewService(store, nil), nil).Handler())
	defer server.Close()
	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("<body>")) || !bytes.Contains(body, []byte("古树迁移保护方案核验台")) {
		t.Fatalf("invalid workbench response: %d", response.StatusCode)
	}
	invalid, err := http.Post(server.URL+"/api/applications", "application/json", strings.NewReader(`{"request_id":"one","unknown":true}`))
	if err != nil {
		t.Fatal(err)
	}
	defer invalid.Body.Close()
	if invalid.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field status = %d", invalid.StatusCode)
	}
}

func TestCreateAndReadApplicationAPI(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewServer(application.NewService(store, nil), nil).Handler())
	defer server.Close()
	payload := map[string]any{"request_id": "web-create", "actor": "编制员", "draft": map[string]any{}}
	raw, _ := json.Marshal(payload)
	response, err := http.Post(server.URL+"/api/applications", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", response.StatusCode)
	}
	var view application.DetailView
	if err := json.NewDecoder(response.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	read, err := http.Get(server.URL + "/api/applications/" + view.Application.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer read.Body.Close()
	if read.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d", read.StatusCode)
	}
}
