package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Config struct {
	Debug bool   `default:"false"`
	Port  string `default:"8080"`
	Env   string `envconfig:"ENV"`
}

var k8sClient kubernetes.Interface
var conf = &Config{}

const (
	BADGE_COLOR_FATAL   = "red"
	BADGE_COLOR_WARN    = "yellow"
	BADGE_COLOR_HEALTHY = "blue"
)

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

	e := echo.New()
	e.GET("/healthz", healthz)
	e.HEAD("/healthz", healthz)
	e.GET("/pods", handlePods)
	e.GET("/nodes", handleNodes)

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		if err := e.Start(":" + conf.Port); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
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

func healthz(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "ok")
}

func handlePods(ctx echo.Context) error {
	pods, err := k8sClient.CoreV1().Pods("").List(ctx.Request().Context(), v1.ListOptions{})
	if err != nil {
		slog.Error(err.Error())
		return ctx.JSON(http.StatusInternalServerError, err.Error())
	}
	healthyPodsCount := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Running" || pod.Status.Phase == "Succeeded" {
			healthyPodsCount++
		}
	}
	var color string
	rate := float64(healthyPodsCount) / float64(len(pods.Items))
	if rate < 0.5 {
		color = BADGE_COLOR_FATAL
	} else if rate < 0.8 {
		color = BADGE_COLOR_WARN
	} else {
		color = BADGE_COLOR_HEALTHY
	}
	return ctx.JSON(http.StatusOK, echo.Map{
		"schemaVersion": 1,
		"label":         fmt.Sprintf("pods(%s)", conf.Env),
		"message":       fmt.Sprintf("%d/%d", healthyPodsCount, len(pods.Items)),
		"color":         color,
	})
}

func handleNodes(ctx echo.Context) error {
	nodes, err := k8sClient.CoreV1().Nodes().List(ctx.Request().Context(), v1.ListOptions{})
	if err != nil {
		slog.Error(err.Error())
		return ctx.JSON(http.StatusInternalServerError, err.Error())
	}
	healthyNodesCount := 0
	for _, node := range nodes.Items {
		conditions := node.Status.Conditions
		if conditions[len(conditions)-1].Status == "True" {
			healthyNodesCount++
		}
	}
	return ctx.JSON(http.StatusOK, echo.Map{
		"schemaVersion": 1,
		"label":         fmt.Sprintf("nodes(%s)", conf.Env),
		"message":       fmt.Sprintf("%d/%d", healthyNodesCount, len(nodes.Items)),
		"color":         "blue",
	})
}
