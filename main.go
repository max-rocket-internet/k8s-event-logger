package main

import (
	"encoding/json"
	"log"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	loggerApplication := log.New(os.Stderr, "", log.LstdFlags)
	loggerEvent := log.New(os.Stdout, "", 0)

	// Using First sample from https://pkg.go.dev/k8s.io/client-go/tools/clientcmd to automatically deal with environment variables and default file paths

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here

	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		loggerApplication.Panicln(err.Error())
	}

	// Note that this *should* automatically sanitize sensitive fields
	loggerApplication.Println("Using configuration:", config.String())

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		loggerApplication.Panicln(err.Error())
	}

	watchlist := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"events",
		corev1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer(
		watchlist,
		&corev1.Event{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				j, _ := json.Marshal(obj)
				loggerEvent.Printf("%s\n", string(j))
			},
		},
	)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)
	select {}
}
