package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Config struct {
	Debug bool   `default:"false"`
	Port  string `default:"8080"`
}

var k8sClient kubernetes.Interface
var conf = &Config{}

func main() {
	godotenv.Load()
	if err := envconfig.Process("APP", conf); err != nil {
		panic(err)
	}
	logLevel := slog.LevelInfo
	if conf.Debug {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})))
	slog.Debug(fmt.Sprintf("conf: %+v", conf))

	var err error
	k8sClient, err = newClient(conf)
	if err != nil {
		panic(err)
	}

	server := http.Server{
		Addr: ":" + conf.Port,
	}
	http.HandleFunc("/healthz", healthz)
	http.HandleFunc("/pods", handlePods)

	slog.Info(fmt.Sprintf("Starting server http://localhost:%s", conf.Port))
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}

func newClient(conf *Config) (kubernetes.Interface, error) {
	var config *rest.Config
	var err error
	if conf.Debug {
		configPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func healthz(w http.ResponseWriter, r *http.Request) {
	slog.Info(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
	w.Write([]byte("ok"))
}

func handlePods(w http.ResponseWriter, r *http.Request) {
	slog.Info(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
	pods, err := k8sClient.CoreV1().Pods("").List(r.Context(), v1.ListOptions{})
	if err != nil {
		slog.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	healthyPodsCount := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Running" || pod.Status.Phase == "Completed" {
			healthyPodsCount++
		}
	}
}
