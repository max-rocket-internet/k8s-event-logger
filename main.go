package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ignoreNormal = flag.Bool("ignore-normal", false, "ignore events of type 'Normal' to reduce noise")
)

func logEvent(obj interface{}, ignoreNormal bool, logger *log.Logger) {
	var eventType string
	switch e := obj.(type) {
	case *corev1.Event:
		eventType = e.Type
	case *eventsv1.Event:
		eventType = e.Type
	default:
		return
	}
	if ignoreNormal && eventType == corev1.EventTypeNormal {
		return
	}
	j, _ := json.Marshal(obj)
	logger.Printf("%s\n", string(j))
}

func main() {
	flag.Parse()

	loggerApplication := log.New(os.Stderr, "", log.LstdFlags)
	loggerEvent := log.New(os.Stdout, "", 0)

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

	handler := func(obj interface{}) {
		logEvent(obj, *ignoreNormal, loggerEvent)
	}

	coreV1watchlist := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"events",
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, coreV1controller := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: coreV1watchlist,
		ObjectType:    &corev1.Event{},
		ResyncPeriod:  5 * time.Minute,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    handler,
			UpdateFunc: func(_, newObj interface{}) { handler(newObj) },
		},
	})

	eventsV1watchlist := cache.NewListWatchFromClient(
		clientset.EventsV1().RESTClient(),
		"events",
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, eventsV1controller := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: eventsV1watchlist,
		ObjectType:    &eventsv1.Event{},
		ResyncPeriod:  5 * time.Minute,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    handler,
			UpdateFunc: func(_, newObj interface{}) { handler(newObj) },
		},
	})

	stop := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	go coreV1controller.Run(stop)
	go eventsV1controller.Run(stop)
	<-sigCh
	close(stop)
}
