package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
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
	switch e := obj.(type) {
	case *corev1.Event:
		if ignoreNormal && e.Type == corev1.EventTypeNormal {
			return
		}
		j, _ := json.Marshal(e)
		logger.Printf("%s\n", string(j))
	case *eventsv1.Event:
		if ignoreNormal && e.Type == corev1.EventTypeNormal {
			return
		}
		j, _ := json.Marshal(e)
		logger.Printf("%s\n", string(j))
	}
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
	loggerApplication.Println("Using configuration:", config.String())

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		loggerApplication.Panicln(err.Error())
	}

	handler := func(obj interface{}) {
		logEvent(obj, *ignoreNormal, loggerEvent)
	}

	// Informer A: core/v1 events
	coreV1watchlist := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"events",
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, coreV1controller := cache.NewInformer(
		coreV1watchlist,
		&corev1.Event{},
		5*time.Minute,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    handler,
			UpdateFunc: func(_, newObj interface{}) { handler(newObj) },
		},
	)

	// Informer B: events.k8s.io/v1 events
	eventsV1watchlist := cache.NewListWatchFromClient(
		clientset.EventsV1().RESTClient(),
		"events",
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, eventsV1controller := cache.NewInformer(
		eventsV1watchlist,
		&eventsv1.Event{},
		5*time.Minute,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    handler,
			UpdateFunc: func(_, newObj interface{}) { handler(newObj) },
		},
	)

	stop := make(chan struct{})
	defer close(stop)
	go coreV1controller.Run(stop)
	go eventsV1controller.Run(stop)
	select {}
}
