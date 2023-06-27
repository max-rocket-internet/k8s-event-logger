package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ignoreNormal = flag.Bool("ignore_normal", false, "ignore events of the normal type, they can be noisy")
)

func main() {
	flag.Parse()

	loggerApplication := log.New(os.Stderr, "", log.LstdFlags)
	loggerEvent := log.New(os.Stdout, "", 0)

	usr, err := user.Current()
	if err != nil {
		loggerApplication.Panicln(err.Error())
	}

	var config *rest.Config

	if k8s_port := os.Getenv("KUBERNETES_PORT"); k8s_port == "" {
		loggerApplication.Println("Using local kubeconfig")
		var kubeconfig string
		home := usr.HomeDir
		if home != "" {
			kubeconfig = fmt.Sprintf("%s/.kube/config", home)
		} else {
			loggerApplication.Panicln("home directory unknown")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			loggerApplication.Panicln(err.Error())
		}
	} else {
		loggerApplication.Println("Using in-cluster authentication")
		config, err = rest.InClusterConfig()
		if err != nil {
			loggerApplication.Panicln(err.Error())
		}
	}

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
				if (*ignoreNormal && obj.(*corev1.Event).Type == corev1.EventTypeNormal) {
					return
				}
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
