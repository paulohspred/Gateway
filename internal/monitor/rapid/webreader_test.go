package rapid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebReaderAuthenticatesAndReadsCurrentData(t *testing.T) {
	var loginCount atomic.Int32
	server := rapidWebTestServer(t, &loginCount, false)
	defer server.Close()

	reader := newTestWebReader(t, server.URL)
	data, err := reader.ReadCurrent(context.Background(), []int{108, 101, 101})
	if err != nil {
		t.Fatal(err)
	}
	if loginCount.Load() != 1 {
		t.Fatalf("expected one login, got %d", loginCount.Load())
	}
	if len(data) != 2 {
		t.Fatalf("expected two unique channels, got %d", len(data))
	}
	if data[0].ChannelNumber != 101 || data[0].Value != 1500 || data[0].Status != 1 {
		t.Fatalf("unexpected first channel: %#v", data[0])
	}
	if data[1].ChannelNumber != 108 || data[1].Value != 60 || data[1].Status != 1 {
		t.Fatalf("unexpected second channel: %#v", data[1])
	}
	if !data[0].ObservedAt.Equal(testNow) {
		t.Fatalf("unexpected observed time %s", data[0].ObservedAt)
	}
}

func TestWebReaderReauthenticatesOnceOnUnauthorized(t *testing.T) {
	var loginCount atomic.Int32
	server := rapidWebTestServer(t, &loginCount, true)
	defer server.Close()

	reader := newTestWebReader(t, server.URL)
	if _, err := reader.ReadCurrent(context.Background(), []int{101}); err != nil {
		t.Fatal(err)
	}
	if loginCount.Load() != 2 {
		t.Fatalf("expected login plus one reauthentication, got %d", loginCount.Load())
	}
}

func TestWebReaderRejectsRemoteRapidEndpoint(t *testing.T) {
	_, err := NewWebReader(WebReaderOptions{
		BaseURL:  "https://example.com/scada/",
		Username: "monitor",
		Password: "secret",
	})
	if err == nil || !strings.Contains(err.Error(), "not loopback") {
		t.Fatalf("expected loopback validation error, got %v", err)
	}
}

func TestWebReaderFailsClosedOnRapidDTOError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Api/Auth/Login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok"})
			writeRapidTestJSON(t, w, map[string]any{"ok": true, "msg": ""})
		case "/Api/Main/GetCurData":
			writeRapidTestJSON(t, w, map[string]any{"ok": false, "msg": "access denied", "data": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	reader := newTestWebReader(t, server.URL)
	if _, err := reader.ReadCurrent(context.Background(), []int{101}); err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("expected DTO failure, got %v", err)
	}
}

func rapidWebTestServer(t *testing.T, loginCount *atomic.Int32, rejectFirstCurrent bool) *httptest.Server {
	t.Helper()
	var currentCount atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Api/Auth/Login":
			if r.Method != http.MethodPost {
				t.Errorf("unexpected login method %s", r.Method)
			}
			var credentials map[string]string
			if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
				t.Errorf("decode credentials: %v", err)
			}
			if credentials["username"] != "monitor" || credentials["password"] != "secret" {
				t.Errorf("unexpected credentials")
			}
			loginCount.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "session", Value: strconv.Itoa(int(loginCount.Load())), Path: "/"})
			writeRapidTestJSON(t, w, map[string]any{"ok": true, "msg": ""})
		case "/Api/Main/GetCurData":
			if _, err := r.Cookie("session"); err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if rejectFirstCurrent && currentCount.Add(1) == 1 {
				http.Error(w, "expired", http.StatusUnauthorized)
				return
			}
			channels := strings.Split(r.URL.Query().Get("cnlNums"), ",")
			points := make([]map[string]any, 0, len(channels))
			for _, channel := range channels {
				switch channel {
				case "101":
					points = append(points, map[string]any{"cnlNum": 101, "val": 1500.0, "stat": 1})
				case "108":
					points = append(points, map[string]any{"cnlNum": 108, "val": 60.0, "stat": 1})
				default:
					t.Errorf("unexpected channel %q", channel)
				}
			}
			writeRapidTestJSON(t, w, map[string]any{"ok": true, "msg": "", "data": points})
		default:
			http.NotFound(w, r)
		}
	}))
}

func newTestWebReader(t *testing.T, baseURL string) *WebReader {
	t.Helper()
	reader, err := NewWebReader(WebReaderOptions{
		BaseURL:  baseURL + "/",
		Username: "monitor",
		Password: "secret",
		Timeout:  2 * time.Second,
		Now:      func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatal(err)
	}
	return reader
}

func writeRapidTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}
