/*
Copyright 2017 Heptio Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// addr tells us what address to have the Prometheus metrics listen on.
var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")

// setup a signal hander to gracefully exit
func sigHandler() <-chan struct{} {
	stop := make(chan struct{})
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c,
			syscall.SIGINT,  // Ctrl+C
			syscall.SIGTERM, // Termination Request
			syscall.SIGSEGV, // FullDerp
			syscall.SIGABRT, // Abnormal termination
			syscall.SIGILL,  // illegal instruction
			syscall.SIGFPE)  // floating point - this is why we can't have nice things
		sig := <-c
		glog.Warningf("Signal (%v) Detected, Shutting Down", sig)
		close(stop)
	}()
	return stop
}

// loadConfig will parse input + config file and return a clientset
func loadConfig() kubernetes.Interface {
	var config *rest.Config
	var err error

	flag.Parse()

	// leverages a file|(ConfigMap)
	// to be located at /etc/eventrouter/config
	viper.SetConfigType("json")
	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/eventrouter/")
	viper.AddConfigPath(".")
	viper.SetDefault("kubeconfig", "")
	viper.SetDefault("sink", "glog")
	viper.SetDefault("resync-interval", time.Minute*30)
	viper.SetDefault("enable-prometheus", true)
	viper.SetDefault("enable-http-pprof", false)
	if err = viper.ReadInConfig(); err != nil {
		panic(err.Error())
	}

	viper.BindEnv("kubeconfig") // Allows the KUBECONFIG env var to override where the kubeconfig is
	viper.BindEnv("WATCH_NAMESPACE")

	// Allow specifying a custom config file via the EVENTROUTER_CONFIG env var
	if forceCfg := os.Getenv("EVENTROUTER_CONFIG"); forceCfg != "" {
		viper.SetConfigFile(forceCfg)
	}
	kubeconfig := viper.GetString("kubeconfig")
	if len(kubeconfig) > 0 {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		panic(err.Error())
	}

	// creates the clientset from kubeconfig
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}

// main entry point of the program
func main() {
	var wg sync.WaitGroup
	clientset := loadConfig()
	sharedInformers := informers.NewSharedInformerFactoryWithOptions(clientset, viper.GetDuration("resync-interval"), informers.WithNamespace(viper.GetString("WATCH_NAMESPACE")))
	eventsInformer := sharedInformers.Core().V1().Events()

	// TODO: Support locking for HA https://github.com/kubernetes/kubernetes/pull/42666
	eventRouter := NewEventRouter(clientset, eventsInformer)
	stop := sigHandler()

	mux := http.NewServeMux()

	// Add handler for /debug/pprof
	if viper.GetBool("enable-http-pprof") {
		glog.Info("Starting http/pprof handler.")

		// copied from net/http/pprof init()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	// Add handler for /metrics
	if viper.GetBool("enable-prometheus") {
		glog.Info("Starting prometheus metrics.")
		mux.Handle("/metrics", promhttp.Handler())
	}

	// Start the http listener for Prometheus Metrics or pprof debugging
	if viper.GetBool("enable-http-pprof") || viper.GetBool("enable-prometheus") {
		go func() {
			glog.Warning(http.ListenAndServe(*addr, mux))
		}()
	}

	// Startup the EventRouter
	wg.Add(1)
	go func() {
		defer wg.Done()
		eventRouter.Run(stop)
	}()

	// Startup the Informer(s)
	glog.Infof("Starting shared Informer(s)")
	sharedInformers.Start(stop)
	wg.Wait()
	glog.Warningf("Exiting main()")
	os.Exit(1)
}
