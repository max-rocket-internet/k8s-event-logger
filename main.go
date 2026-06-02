package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ignoreNormal = flag.Bool("ignore-normal", false, "ignore events of type 'Normal' to reduce noise")
)

func logEvent(obj any, ignoreNormal bool, logger *log.Logger) {
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

	handler := func(obj any) {
		logEvent(obj, *ignoreNormal, loggerEvent)
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
}
