package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	eventsv1 "k8s.io/api/events/v1"
	clientmetrics "k8s.io/client-go/tools/metrics"
)

// val returns the current value of the counter with the given label values.
func val(c *prometheus.CounterVec, lv ...string) float64 {
	return testutil.ToFloat64(c.WithLabelValues(lv...))
}

func TestLogEvent_EventsV1_Normal_Ignored(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	logEvent(&eventsv1.Event{Type: "Normal"}, true, logger, newMetrics("test"))
	if buf.Len() != 0 {
		t.Errorf("expected no output for ignored Normal event, got %q", buf.String())
	}
}

func TestLogEvent_EventsV1_Normal_NotIgnored(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	logEvent(&eventsv1.Event{Type: "Normal"}, false, logger, newMetrics("test"))
	if buf.Len() == 0 {
		t.Error("expected output for Normal event when ignoreNormal=false")
	}
}

func TestLogEvent_EventsV1_Warning_AlwaysLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	logEvent(&eventsv1.Event{Type: "Warning"}, true, logger, newMetrics("test"))
	if buf.Len() == 0 {
		t.Error("expected output for Warning event even when ignoreNormal=true")
	}
}

func TestLogEvent_CountsEventsAndBytesByType(t *testing.T) {
	var buf bytes.Buffer
	m, logger := newMetrics("test"), log.New(&buf, "", 0)

	logEvent(&eventsv1.Event{Type: "Normal"}, false, logger, m)
	normalBytes := buf.Len()
	logEvent(&eventsv1.Event{Type: "Warning"}, false, logger, m)
	warningBytes := buf.Len() - normalBytes

	if got := val(m.events, "Normal"); got != 1 {
		t.Errorf("events_total{type=Normal} = %v, want 1", got)
	}
	if got := val(m.events, "Warning"); got != 1 {
		t.Errorf("events_total{type=Warning} = %v, want 1", got)
	}
	if got := val(m.writtenBytes, "Normal"); got != float64(normalBytes) {
		t.Errorf("written_bytes_total{type=Normal} = %v, want %d", got, normalBytes)
	}
	if got := val(m.writtenBytes, "Warning"); got != float64(warningBytes) {
		t.Errorf("written_bytes_total{type=Warning} = %v, want %d", got, warningBytes)
	}
}

func TestLogEvent_IgnoredNormal_NotCounted(t *testing.T) {
	var buf bytes.Buffer
	m, logger := newMetrics("test"), log.New(&buf, "", 0)

	logEvent(&eventsv1.Event{Type: "Normal"}, true, logger, m)

	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
	if got := val(m.events, "Normal"); got != 0 {
		t.Errorf("events_total{type=Normal} = %v, want 0", got)
	}
	if got := val(m.writtenBytes, "Normal"); got != 0 {
		t.Errorf("written_bytes_total{type=Normal} = %v, want 0", got)
	}
}

func TestLogEvent_UnknownType_CountedAsOther(t *testing.T) {
	var buf bytes.Buffer
	m, logger := newMetrics("test"), log.New(&buf, "", 0)

	logEvent(&eventsv1.Event{Type: ""}, false, logger, m)
	logEvent(&eventsv1.Event{Type: "Weird"}, false, logger, m)

	if got := val(m.events, "Other"); got != 2 {
		t.Errorf("events_total{type=Other} = %v, want 2", got)
	}
	if got := testutil.CollectAndCount(m.events); got != 3 {
		t.Errorf("events_total has %d series, want 3 (Normal, Warning, Other)", got)
	}
}

func TestLogEvent_NonEventObject_Ignored(t *testing.T) {
	var buf bytes.Buffer
	m, logger := newMetrics("test"), log.New(&buf, "", 0)

	logEvent("not an event", false, logger, m)

	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
	if got := testutil.CollectAndCount(m.events); got != 2 {
		t.Errorf("events_total has %d series, want 2", got)
	}
	if got := val(m.events, "Normal") + val(m.events, "Warning"); got != 0 {
		t.Errorf("events_total sum = %v, want 0", got)
	}
	if got := testutil.CollectAndCount(m.writtenBytes); got != 2 {
		t.Errorf("written_bytes_total has %d series, want 2", got)
	}
	if got := val(m.writtenBytes, "Normal") + val(m.writtenBytes, "Warning"); got != 0 {
		t.Errorf("written_bytes_total sum = %v, want 0", got)
	}
}

func TestNewMetrics_BuildInfo(t *testing.T) {
	m := newMetrics("1.2.3")

	if got := testutil.ToFloat64(m.buildInfo.WithLabelValues("1.2.3", runtime.Version())); got != 1 {
		t.Errorf("build_info{version=1.2.3} = %v, want 1", got)
	}
	if got := testutil.CollectAndCount(m.buildInfo); got != 1 {
		t.Errorf("build_info has %d series, want 1", got)
	}
}

func TestNewMetrics_SeriesStartAtZero(t *testing.T) {
	m := newMetrics("test")

	if got := testutil.CollectAndCount(m.events); got != 2 {
		t.Errorf("events_total has %d series at startup, want 2 (Normal, Warning)", got)
	}
	if got := testutil.CollectAndCount(m.writtenBytes); got != 2 {
		t.Errorf("written_bytes_total has %d series at startup, want 2 (Normal, Warning)", got)
	}
}

func TestRequestResult_Increment(t *testing.T) {
	m := newMetrics("test")
	var rm clientmetrics.ResultMetric = requestResult{m.requests}

	rm.Increment(context.Background(), "200", "GET", "h:443")
	rm.Increment(context.Background(), "<error>", "GET", "h:443")

	if got := val(m.requests, "200", "GET", "h:443"); got != 1 {
		t.Errorf(`requests_total{code="200"} = %v, want 1`, got)
	}
	if got := val(m.requests, "<error>", "GET", "h:443"); got != 1 {
		t.Errorf(`requests_total{code="<error>"} = %v, want 1`, got)
	}
}

// get sends a request to h and returns the status code and the body.
func get(t *testing.T, h http.Handler, method, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	return rec.Code, string(body)
}

func TestNewMetricsServer_DisabledWhenAddressEmpty(t *testing.T) {
	if srv := newMetricsServer("", newMetrics("test").registry); srv != nil {
		t.Errorf("newMetricsServer(\"\") = %+v, want nil", srv)
	}
}

func TestMetricsHandler_ServesFamilies(t *testing.T) {
	m := newMetrics("test")
	h := newMetricsServer("127.0.0.1:0", m.registry).Handler

	code, body := get(t, h, http.MethodGet, "/metrics")
	if code != http.StatusOK {
		t.Fatalf("GET /metrics: status %d, want 200", code)
	}
	want := []string{
		"go_goroutines ",
		`k8s_event_logger_build_info{goversion="`,
		`k8s_event_logger_events_total{type="Normal"} 0`,
		`k8s_event_logger_events_total{type="Warning"} 0`,
		`k8s_event_logger_written_bytes_total{type="Normal"} 0`,
		`k8s_event_logger_written_bytes_total{type="Warning"} 0`,
	}
	if runtime.GOOS == "linux" {
		want = append(want, "process_cpu_seconds_total ")
	}
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("GET /metrics: body does not contain %q", w)
		}
	}
	if strings.Contains(body, "rest_client_requests_total") {
		t.Error("GET /metrics: rest_client_requests_total present before any request was counted")
	}

	requestResult{m.requests}.Increment(context.Background(), "200", "GET", "h")
	_, body = get(t, h, http.MethodGet, "/metrics")
	if w := `rest_client_requests_total{code="200",host="h",method="GET"} 1`; !strings.Contains(body, w) {
		t.Errorf("GET /metrics: body does not contain %q", w)
	}
}

func TestMetricsHandler_OtherPathsAndMethods(t *testing.T) {
	h := newMetricsServer("127.0.0.1:0", newMetrics("test").registry).Handler

	if code, _ := get(t, h, http.MethodGet, "/"); code != http.StatusNotFound {
		t.Errorf("GET /: status %d, want 404", code)
	}
	if code, _ := get(t, h, http.MethodPost, "/metrics"); code != http.StatusMethodNotAllowed {
		t.Errorf("POST /metrics: status %d, want 405", code)
	}
}
