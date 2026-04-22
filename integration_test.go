package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
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
