package staticrange_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/web"
)

func TestRepeatedStaticRangeRequestsKeepResourceReadable(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := web.NewServer(application.NewService(store, nil), nil).Handler()

	requestRange := func() (int, string) {
		request := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
		request.Header.Set("Range", "bytes=0-31")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		result := response.Result()
		defer result.Body.Close()
		body, readErr := io.ReadAll(result.Body)
		if readErr != nil {
			t.Fatalf("read range response: %v", readErr)
		}
		return result.StatusCode, string(body)
	}

	firstStatus, firstBody := requestRange()
	if firstStatus != http.StatusPartialContent || firstBody == "" {
		t.Fatalf("first range status=%d body=%q", firstStatus, firstBody)
	}
	secondStatus, secondBody := requestRange()
	if secondStatus != http.StatusPartialContent || secondBody != firstBody {
		t.Fatalf("cached range resource became unreadable: first=(%d,%q) second=(%d,%q)", firstStatus, firstBody, secondStatus, secondBody)
	}
}
