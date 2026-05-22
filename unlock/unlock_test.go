package unlock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckOpenAI_Accessible(t *testing.T) {
	svc := services[0]
	if !svc.Check(401, `{"error": {"message": "Invalid API key"}}`) {
		t.Error("expected OpenAI accessible with 401")
	}
}

func TestCheckOpenAI_Blocked(t *testing.T) {
	svc := services[0]
	if svc.Check(403, `{"error": {"message": "unsupported_country"}}`) {
		t.Error("expected OpenAI blocked with 403 + unsupported_country")
	}
}

func TestCheckOpenAI_403Other(t *testing.T) {
	svc := services[0]
	if svc.Check(403, `{"error": {"message": "something else"}}`) {
		t.Error("expected OpenAI blocked with 403 + other reason")
	}
}

func TestCheckClaude_Accessible(t *testing.T) {
	svc := services[1]
	if !svc.Check(401, `{"error": {"message": "invalid x-api-key"}}`) {
		t.Error("expected Claude accessible with 401")
	}
}

func TestCheckClaude_Blocked(t *testing.T) {
	svc := services[1]
	if svc.Check(403, `{"error": {"message": "forbidden"}}`) {
		t.Error("expected Claude blocked with 403")
	}
}

func TestCheckCodex_Accessible(t *testing.T) {
	svc := services[2]
	if !svc.Check(200, `<html>codex</html>`) {
		t.Error("expected Codex accessible with 200")
	}
}

func TestCheckCodex_Blocked(t *testing.T) {
	svc := services[2]
	if svc.Check(403, `<html>blocked</html>`) {
		t.Error("expected Codex blocked with 403")
	}
}

func TestCheckService_OpenAI_HTTPServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer srv.Close()

	svc := serviceCheck{Name: "OpenAI", URL: srv.URL, Method: "GET", Check: services[0].Check}
	client := srv.Client()
	unlocked := checkService(context.Background(), client, svc)
	if !unlocked {
		t.Error("expected OpenAI to be unlocked via HTTP server")
	}
}

func TestCheckService_Claude_POST(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") == "" {
			t.Error("expected x-api-key header")
		}
		w.WriteHeader(401)
		w.Write([]byte(`{"error": {"message": "invalid x-api-key"}}`))
	}))
	defer srv.Close()

	svc := serviceCheck{
		Name:    "Claude",
		URL:     srv.URL,
		Method:  "POST",
		Headers: map[string]string{"x-api-key": "test", "content-type": "application/json"},
		Check:   services[1].Check,
	}
	client := srv.Client()
	unlocked := checkService(context.Background(), client, svc)
	if !unlocked {
		t.Error("expected Claude to be unlocked via HTTP server")
	}
}

func TestCheckService_ConnectionError(t *testing.T) {
	svc := serviceCheck{Name: "OpenAI", URL: "http://127.0.0.1:1", Method: "GET", Check: services[0].Check}
	client := &http.Client{Timeout: 1 * 1e9}
	unlocked := checkService(context.Background(), client, svc)
	if unlocked {
		t.Error("expected service to not be unlocked on connection error")
	}
}
