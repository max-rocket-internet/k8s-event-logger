package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	clientmetrics "k8s.io/client-go/tools/metrics"
)

// version is set at build time with -ldflags "-X main.version=<version>".
var version = "dev"

var (
	ignoreNormal   = flag.Bool("ignore-normal", false, "ignore events of type 'Normal' to reduce noise")
	metricsAddress = flag.String("metrics-address", "", "address to serve Prometheus metrics on, e.g. ':8080'; empty disables the metrics server")
)

// metrics holds the Prometheus registry and the collectors of this program.
type metrics struct {
	registry     *prometheus.Registry
	buildInfo    *prometheus.GaugeVec
	events       *prometheus.CounterVec
	writtenBytes *prometheus.CounterVec
	requests     *prometheus.CounterVec
}

func newMetrics(version string) *metrics {
	m := &metrics{
		registry: prometheus.NewRegistry(),
		buildInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "k8s_event_logger_build_info",
			Help: "Build information of k8s-event-logger. The value is always 1.",
		}, []string{"version", "goversion"}),
		events: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "k8s_event_logger_events_total",
			Help: "Number of events written to stdout, by event type.",
		}, []string{"type"}),
		writtenBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "k8s_event_logger_written_bytes_total",
			Help: "Number of bytes written to stdout for events, by event type.",
		}, []string{"type"}),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rest_client_requests_total",
			Help: "Number of HTTP requests to the Kubernetes API, partitioned by status code, method, and host.",
		}, []string{"code", "method", "host"}),
	}
	m.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.buildInfo,
		m.events,
		m.writtenBytes,
		m.requests,
	)

	m.buildInfo.WithLabelValues(version, runtime.Version()).Set(1)
	// Create the common series up front so that they are exported as 0
	// before the first event.
	for _, t := range []string{"Normal", "Warning"} {
		m.events.WithLabelValues(t)
		m.writtenBytes.WithLabelValues(t)
	}

	return m
}

// eventTypeLabel maps an event type to a bounded set of label values.
func eventTypeLabel(t string) string {
	switch t {
	case "Normal", "Warning":
		return t
	default:
		return "Other"
	}
}

// requestResult counts client-go requests by result. It implements
// k8s.io/client-go/tools/metrics.ResultMetric.
type requestResult struct {
	counter *prometheus.CounterVec
}

func (r requestResult) Increment(_ context.Context, code, method, host string) {
	r.counter.WithLabelValues(code, method, host).Inc()
}

// newMetricsServer returns an HTTP server that serves reg on /metrics, or nil
// when addr is empty.
func newMetricsServer(addr string, reg *prometheus.Registry) *http.Server {
	if addr == "" {
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func logEvent(obj any, ignoreNormal bool, logger *log.Logger, m *metrics) {
	event, ok := obj.(*eventsv1.Event)
	if !ok {
		return
	}
	if ignoreNormal && event.Type == "Normal" {
		return
	}

	event.ManagedFields = nil

	j, _ := json.Marshal(event)
	logger.Printf("%s\n", string(j))

	t := eventTypeLabel(event.Type)
	m.events.WithLabelValues(t).Inc()
	m.writtenBytes.WithLabelValues(t).Add(float64(len(j) + 1)) // +1 for the newline
}

func main() {
	flag.Parse()

	loggerApplication := log.New(os.Stderr, "", log.LstdFlags)
	loggerEvent := log.New(os.Stdout, "", 0)

	m := newMetrics(version)
	// Must be registered before the first request is made; it can only be done once.
	clientmetrics.Register(clientmetrics.RegisterOpts{
		RequestResult: requestResult{m.requests},
	})

	srv := newMetricsServer(*metricsAddress, m.registry)
	if srv != nil {
		ln, err := net.Listen("tcp", srv.Addr)
		if err != nil {
			loggerApplication.Fatalln(err)
		}
		go func() {
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				loggerApplication.Println("metrics server:", err)
			}
		}()
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		loggerApplication.Panicln(err.Error())
	}

	// loggerApplication.Println("Using configuration:", config.String())

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		loggerApplication.Panicln(err.Error())
	}

	handler := func(obj any) {
		logEvent(obj, *ignoreNormal, loggerEvent, m)
	}

	eventsV1watchlist := cache.NewListWatchFromClient(
		clientset.EventsV1().RESTClient(),
		"events",
		"",
		fields.Everything(),
	)
	_, eventsV1controller := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: eventsV1watchlist,
		ObjectType:    &eventsv1.Event{},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: handler,
		},
	})

	stop := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	go eventsV1controller.Run(stop)
	<-sigCh
	close(stop)

	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			loggerApplication.Println("metrics server shutdown:", err)
		}
	}
}
