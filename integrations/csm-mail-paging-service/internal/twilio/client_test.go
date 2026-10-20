package twilio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMakeCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, _ := r.BasicAuth()
		if r.URL.Path != "/Accounts/AC1/Calls.json" || u != "AC1" || p != "tok" {
			t.Errorf("unexpected request %s auth %s", r.URL.Path, u)
		}
		_ = r.ParseForm()
		if r.Form.Get("To") != "+15550100" || r.Form.Get("From") != "+15550199" || r.Form.Get("Timeout") != "20" {
			t.Errorf("unexpected form %v", r.Form)
		}
		if tw := r.Form.Get("Twiml"); !strings.Contains(tw, "&lt;Dial&gt;") || strings.Contains(tw, "<Dial>") {
			t.Errorf("message was not escaped: %s", tw)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sid":"CA1","status":"queued"}`))
	}))
	defer srv.Close()
	c, err := New(Config{AccountSID: "AC1", AuthToken: "tok", FromNumber: "+15550199", RingTimeoutSeconds: 20, APIBaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	call, err := c.MakeCall(context.Background(), "+15550100", "down <Dial>")
	if err != nil || call.SID != "CA1" {
		t.Fatalf("call=%+v err=%v", call, err)
	}
}

func TestMakeCallRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("+15550100 invalid"))
	}))
	defer srv.Close()
	c, _ := New(Config{AccountSID: "AC1", AuthToken: "t", FromNumber: "+1", APIBaseURL: srv.URL})
	_, err := c.MakeCall(context.Background(), "+15550100", "x")
	if err == nil || strings.Contains(err.Error(), "+15550100") {
		t.Errorf("expected an error without the number, got %v", err)
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Error("expected error")
	}
}
