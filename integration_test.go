package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// runEventLogger runs the event logger with the given arguments and returns the stdout, stderr, and any error
func runEventLogger(t *testing.T, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", append([]string{"run", "main.go"}, args...)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.Logf("Running event logger with args: %v", args)

	// Start the event logger
	err := cmd.Start()
	if err != nil {
		return "", "", err
	}

	// Give the event logger time to start up
	time.Sleep(2 * time.Second)

	// Create a Kubernetes client
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("KUBECONFIG environment variable not set")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		cmd.Process.Kill()
		return "", "", err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		cmd.Process.Kill()
		return "", "", err
	}

	// Create a test namespace with a unique name
	nsName := "event-logger-test-" + time.Now().Format("150405")
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}
	_, err = clientset.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil {
		cmd.Process.Kill()
		return "", "", err
	}
	defer clientset.CoreV1().Namespaces().Delete(context.Background(), ns.Name, metav1.DeleteOptions{})

	// Create a test pod to generate events
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "busybox",
					Command: []string{
						"sleep",
						"10",
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	_, err = clientset.CoreV1().Pods(ns.Name).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		cmd.Process.Kill()
		return "", "", err
	}

	// Wait for events to be generated and captured
	time.Sleep(5 * time.Second)

	// Kill the event logger process
	if err := cmd.Process.Kill(); err != nil {
		t.Logf("Failed to kill event logger process: %v", err)
	}

	return stdout.String(), stderr.String(), nil
}

// logEventOutput logs the event output with a limit on the number of events shown
func logEventOutput(t *testing.T, output string, title string) {
	if output == "" {
		t.Logf("%s: No events captured", title)
		return
	}

	// Count and log events
	lines := bytes.Split([]byte(output), []byte("\n"))
	var eventCount int
	for _, line := range lines {
		if len(line) > 0 {
			eventCount++
		}
	}

	t.Logf("%s: %d events found", title, eventCount)

	// Print the captured events for debugging (limit to first 5 for readability)
	maxEvents := 5
	for i, line := range lines {
		if len(line) > 0 {
			if i < maxEvents {
				t.Logf("  Event %d: %s", i+1, string(line))
			} else if i == maxEvents {
				t.Logf("  ... and %d more events", eventCount-maxEvents)
				break
			}
		}
	}
}

// TestIntegration runs the event logger against a real Kubernetes cluster
// and verifies that it can receive events.
// This test requires a running Kubernetes cluster and KUBECONFIG to be set.
// It is intended to be run in CI with a Kind cluster.
func TestIntegration(t *testing.T) {
	// Ensure we have a valid kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("KUBECONFIG environment variable not set")
	}

	// Test 1: Run without -ignore-normal flag
	t.Run("WithAllEvents", func(t *testing.T) {
		stdout, stderr, err := runEventLogger(t)
		if err != nil {
			t.Fatalf("Failed to run event logger: %v", err)
		}

		if stdout == "" {
			t.Errorf("No events were captured by the event logger")
			t.Logf("Stderr: %s", stderr)
		} else {
			logEventOutput(t, stdout, "All events")

			// Print stderr for additional debugging context
			if stderr != "" {
				t.Logf("Stderr output: %s", stderr)
			}
		}
	})

	// Test 2: Run with -ignore-normal flag
	t.Run("IgnoringNormalEvents", func(t *testing.T) {
		stdout, stderr, err := runEventLogger(t, "-ignore-normal")
		if err != nil {
			t.Fatalf("Failed to run event logger: %v", err)
		}

		// It's okay if we don't capture any events when ignoring normal events
		// as most events in a test cluster are Normal
		logEventOutput(t, stdout, "Non-normal events only")

		// Print stderr for additional debugging context
		if stderr != "" {
			t.Logf("Stderr output: %s", stderr)
		}
	})
}

// buildBinary builds the event logger into a temporary directory and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "k8s-event-logger")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// TestMetricsAddressInUse_ExitsNonZero checks that the event logger exits with an
// error when it cannot listen on the metrics address. It needs no cluster.
func TestMetricsAddressInUse_ExitsNonZero(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	bin := buildBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "-metrics-address="+ln.Addr().String())
	cmd.Stderr = &stderr
	err = cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected the event logger to exit with an error, got %v (stderr: %s)", err, stderr.String())
	}
	if code := exitErr.ExitCode(); code != 1 {
		t.Errorf("exit code %d, want 1 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "address already in use") {
		t.Errorf("stderr does not mention the bind error: %s", stderr.String())
	}
}

func TestMetricsAddressMalformed_ExitsNonZero(t *testing.T) {
	bin := buildBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "-metrics-address=foo")
	cmd.Stderr = &stderr
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected the event logger to exit with an error, got %v (stderr: %s)", err, stderr.String())
	}
	if code := exitErr.ExitCode(); code != 1 {
		t.Errorf("exit code %d, want 1 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "missing port in address") {
		t.Errorf("stderr does not mention the address error: %s", stderr.String())
	}
}

// metricValue returns the sum of the values of all lines in a Prometheus text
// exposition that match re, and whether any line matched.
func metricValue(exposition string, re *regexp.Regexp) (float64, bool) {
	var sum float64
	var found bool
	for _, m := range re.FindAllStringSubmatch(exposition, -1) {
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		sum += v
		found = true
	}
	return sum, found
}

var (
	reWarningEvents = regexp.MustCompile(`(?m)^k8s_event_logger_events_total\{type="Warning"\} (\S+)$`)
	reWarningBytes  = regexp.MustCompile(`(?m)^k8s_event_logger_written_bytes_total\{type="Warning"\} (\S+)$`)
	reGet200        = regexp.MustCompile(`(?m)^rest_client_requests_total\{code="200",host="[^"]*",method="GET"\} (\S+)$`)
	reBuildInfoDev  = regexp.MustCompile(`(?m)^k8s_event_logger_build_info\{goversion="[^"]+",version="dev"\} (\S+)$`)
)

// TestIntegrationMetrics runs the event logger with -metrics-address against a
// real Kubernetes cluster and checks the metrics it serves.
// This test requires a running Kubernetes cluster and KUBECONFIG to be set.
func TestIntegrationMetrics(t *testing.T) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("KUBECONFIG environment variable not set")
	}

	// Pick a free loopback address.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	bin := buildBinary(t)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "-metrics-address="+addr)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the event logger: %v", err)
	}
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	t.Cleanup(stop)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("loading kubeconfig: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	event := &eventsv1.Event{
		ObjectMeta:          metav1.ObjectMeta{GenerateName: "event-logger-metrics-test-"},
		EventTime:           metav1.NowMicro(),
		ReportingController: "k8s-event-logger-test",
		ReportingInstance:   "test",
		Action:              "Testing",
		Reason:              "MetricsTest",
		Type:                "Warning",
		Note:                "created by TestIntegrationMetrics",
		Regarding: corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Namespace",
			Name:       "default",
		},
	}
	created, err := clientset.EventsV1().Events("default").Create(context.Background(), event, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("creating event: %v", err)
	}
	t.Cleanup(func() {
		_ = clientset.EventsV1().Events("default").Delete(context.Background(), created.Name, metav1.DeleteOptions{})
	})

	url := fmt.Sprintf("http://%s/metrics", addr)
	var last string
	var problems []string
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		resp, err := http.Get(url)
		if err != nil {
			problems = []string{fmt.Sprintf("GET %s: %v", url, err)}
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		last = string(body)

		problems = nil
		if v, _ := metricValue(last, reWarningEvents); v < 1 {
			problems = append(problems, fmt.Sprintf(`k8s_event_logger_events_total{type="Warning"} = %v, want >= 1`, v))
		}
		if v, _ := metricValue(last, reWarningBytes); v <= 0 {
			problems = append(problems, fmt.Sprintf(`k8s_event_logger_written_bytes_total{type="Warning"} = %v, want > 0`, v))
		}
		if v, _ := metricValue(last, reGet200); v < 2 {
			problems = append(problems, fmt.Sprintf(`sum(rest_client_requests_total{code="200",method="GET"}) = %v, want >= 2`, v))
		}
		if v, ok := metricValue(last, reBuildInfoDev); !ok || v != 1 {
			problems = append(problems, `k8s_event_logger_build_info{version="dev"} 1 not found`)
		}
		if len(problems) == 0 {
			break
		}
	}
	if len(problems) > 0 {
		t.Fatalf("metrics not as expected after 30s:\n  %s\nlast scrape:\n%s\nstderr:\n%s",
			strings.Join(problems, "\n  "), last, stderr.String())
	}

	stop()
	if !strings.Contains(stdout.String(), `"reason":"MetricsTest"`) {
		t.Errorf("stdout does not contain the test event; stderr:\n%s", stderr.String())
	}
}
