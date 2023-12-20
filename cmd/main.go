package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/moshevayner/go-k8s-http-api-interface/internal/handlers"

	"crypto/tls"
	"crypto/x509"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up a channel to listen for the interrupt signal
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt)

	// Run the servers and manager
	err := run(os.Args, stopCh, ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", err)
		os.Exit(1)
	}

	// Wait for the interrupt signal
	<-stopCh
	klog.Info("Shutting down...")

	// Initiate a graceful shutdown
	cancel()

}

func setupManager() (ctrl.Manager, error) {
	scheme := runtime.NewScheme()
	// Register the apps/v1 group of the Kubernetes API with the scheme
	if err := appsv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add apps/v1 to scheme: %w", err)
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:   scheme,
		NewCache: cache.New,
		Metrics:  metricsserver.Options{BindAddress: "0"},
		Logger:   ctrl.Log.WithName("controller-runtime"),
	})
	if err != nil {
		return nil, err
	}
	return mgr, nil
}

// loggingMiddleware returns a new http.HandlerFunc that wraps the provided handler
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log the request
		klog.V(5).Infof("Started %s %s", r.Method, r.URL.Path)

		next.ServeHTTP(w, r)

		// Log the response time
		klog.V(5).Infof("Completed in %v", time.Since(start))
	}
}

func run(args []string, stopCh chan os.Signal, ctx context.Context) error {
	// Get the user's home directory
	homedir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Parse command line flags
	var port, kubeconfig, serverCert, certKey, caCert string
	flagSet := flag.NewFlagSet(args[0], flag.ExitOnError)
	flagSet.StringVar(&port, "port", "8443", "server port")
	flagSet.StringVar(&kubeconfig, "kubeconfig", filepath.Join(homedir, ".kube", "config"), "path to the kubeconfig file")
	flagSet.StringVar(&serverCert, "server-cert", "", "path to the server certificate")
	flagSet.StringVar(&certKey, "cert-key", "", "path to the certificate key")
	flagSet.StringVar(&caCert, "ca-cert", "", "path to the CA certificate")
	klog.InitFlags(flagSet)
	defer klog.Flush()
	err = flagSet.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	// Load server's certificate and private key
	cert, err := tls.LoadX509KeyPair(serverCert, certKey)
	if err != nil {
		klog.Fatalf("Error loading server certificate and private key: %v", err)
	}

	// Parse the provided CA certificate
	caCertPool := x509.NewCertPool()
	parsedCaCert, err := os.ReadFile(caCert)
	if err != nil {
		klog.Fatalf("Error loading CA certificate: %v", err)
	}
	caCertPool.AppendCertsFromPEM([]byte(parsedCaCert))

	// Create a tls.Config with the server certificate and require client cert verification
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS13,
	}
	server := &http.Server{
		Addr:      ":" + port,
		Handler:   nil, // use http.DefaultServeMux
		TLSConfig: tlsConfig,
	}
	if err != nil {
		klog.Fatalf("Error loading server certificate: %v", err)
	}

	// use the current context in kubeconfig

	var config *rest.Config

	// First, try to load the kubeconfig file
	klog.V(5).Infof("Trying to load kubeconfig file: %v", kubeconfig)
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		// log the error as a warning, and try to get the in-cluster config
		klog.Warningf("Error loading kubeconfig file: %v", err)
		klog.V(5).Info("Trying to get in-cluster config")
		config, err = rest.InClusterConfig()
		if err != nil {
			if err == rest.ErrNotInCluster {
				// since kubeconfig failed to load and we're not in cluster, we can't continue
				klog.Fatalf("kubeconfig failed to load and not running in cluster, cannot continue. Please provide a valid kubeconfig file or run in-cluster")
			} else {
				// we are running in-cluster, but there was an error getting the config (other than ErrNotInCluster)
				klog.Fatalf("Error getting in-cluster config: %v", err)
			}
		} else {
			klog.Info("Using in-cluster config")
		}
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// Create a new manager to watch for changes to deployments
	mgr, err := setupManager()
	if err != nil {
		klog.Fatalf("Error setting up manager: %v", err)
	}

	// HealthzHandler is an HTTP handler for the healthz API.
	healthzHandler := &handlers.HealthzHandler{Client: clientset.RESTClient()}
	http.Handle("/healthz", healthzHandler)

	// DeploymentsHandler is an HTTP handler for the deployments API.
	// This handler uses the manager's client to interact with the Kubernetes API, in order to take advantage of the cache.
	deploymentsHandler := &handlers.DeploymentsHandler{
		Client: mgr.GetClient(),
	}

	http.HandleFunc("/deployments", loggingMiddleware(deploymentsHandler.ListDeployments))

	// This is a quick and dirty way to handle the two different methods for the /deployments/{namespace}/{deployment}/replicas endpoint
	// As we add more handlers, we may want to use a router instead of a switch statement. This does not scale well and isn't really production-ready.
	http.HandleFunc("/deployments/", loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			deploymentsHandler.GetDeploymentReplicas(w, r)
		case http.MethodPut:
			deploymentsHandler.SetDeploymentReplicas(w, r)
		default:
			// return a 405 Method Not Allowed
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	// Unauthenticated server setup
	healthzServer := &http.Server{
		Addr: ":8080", // Use a different port for unauthenticated server
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" {
				// Serve /healthz requests
				healthzHandler.ServeHTTP(w, r)
			} else {
				// Return a 404 for all other requests
				// TODO in a future iteration, we may want to return a redirect to the authenticated server
				w.WriteHeader(http.StatusNotFound)
			}
		}),
	}

	// Start the controller-manager in a separate goroutine
	go func() {
		if err := mgr.Start(ctx); err != nil {
			klog.Fatalf("Problem running manager: %v", err)
		}
	}()

	// Start the main server in a separate goroutine
	go func() {
		klog.Info("Starting main server...")
		klog.V(5).Infof("TLS port: %s", port)
		defer klog.Flush()

		if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			klog.Fatalf("Error starting main server: %v", err)
		}
	}()

	// Start the unauthenticated server for the healthz API in a separate goroutine
	go func() {
		klog.Info("Starting healthz server...")
		klog.V(5).Info("healthz port: 8080")
		defer klog.Flush()

		err := healthzServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			klog.Fatalf("Error starting healthz server: %v", err)
		}
	}()

	// Shutdown logic
	go func() {
		<-ctx.Done() // Wait for shutdown signal

		// Create a new context for shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		// Shutdown the main server
		if err := server.Shutdown(shutdownCtx); err != nil {
			klog.Errorf("Error shutting down main server: %v", err)
		}

		// Shutdown the healthz server
		if err := healthzServer.Shutdown(shutdownCtx); err != nil {
			klog.Errorf("Error shutting down healthz server: %v", err)
		}
	}()

	return nil
}
