package main

import (
	"encoding/json"
	"fmt"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"os/user"
	"time"
)

func main() {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	var config *rest.Config

	if k8s_port := os.Getenv("KUBERNETES_PORT"); k8s_port == "" {
		fmt.Println("Using local kubeconfig")
		var kubeconfig string
		home := usr.HomeDir
		if home != "" {
			kubeconfig = fmt.Sprintf("%s/.kube/config", home)
		} else {
			panic("home directory unknown")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	} else {
		fmt.Println("Using in cluster authentication")
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	watchlist := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"events",
		v1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Event{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				j, _ := json.Marshal(obj)
				fmt.Printf("%s\n", string(j))
			},
		},
	)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)
	for {
		time.Sleep(time.Second)
	}

}
