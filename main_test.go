package main

import (
	"bytes"
	"log"
	"testing"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
)

func TestLogEvent_CoreV1_Normal_Ignored(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	logEvent(&corev1.Event{Type: corev1.EventTypeNormal}, true, logger)
	if buf.Len() != 0 {
		t.Errorf("expected no output for ignored Normal event, got %q", buf.String())
	}
}

func TestLogEvent_CoreV1_Normal_NotIgnored(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	logEvent(&corev1.Event{Type: corev1.EventTypeNormal}, false, logger)
	if buf.Len() == 0 {
		t.Error("expected output for Normal event when ignoreNormal=false")
	}
}

func TestLogEvent_CoreV1_Warning_AlwaysLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	logEvent(&corev1.Event{Type: corev1.EventTypeWarning}, true, logger)
	if buf.Len() == 0 {
		t.Error("expected output for Warning event even when ignoreNormal=true")
	}
}

func TestLogEvent_EventsV1_Normal_Ignored(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	logEvent(&eventsv1.Event{Type: "Normal"}, true, logger)
	if buf.Len() != 0 {
		t.Errorf("expected no output for ignored Normal event, got %q", buf.String())
	}
}

func TestLogEvent_EventsV1_Normal_NotIgnored(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	logEvent(&eventsv1.Event{Type: "Normal"}, false, logger)
	if buf.Len() == 0 {
		t.Error("expected output for Normal event when ignoreNormal=false")
	}
}

func TestLogEvent_EventsV1_Warning_AlwaysLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	logEvent(&eventsv1.Event{Type: "Warning"}, true, logger)
	if buf.Len() == 0 {
		t.Error("expected output for Warning event even when ignoreNormal=true")
	}
}
