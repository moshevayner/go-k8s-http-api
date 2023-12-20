package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

type healthResponse struct {
	Status string `json:"status"`
}

// HealthzHandler is an HTTP handler for the healthz API.
type HealthzHandler struct {
	Client rest.Interface
}

func (h *HealthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Query the /healthz endpoint of the Kubernetes API
	result := h.Client.Get().AbsPath("/healthz").Do(r.Context())
	err := result.Error()

	// Prepare the response header
	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		// Send an error response
		w.WriteHeader(http.StatusServiceUnavailable)
		encErr := json.NewEncoder(w).Encode(healthResponse{Status: fmt.Sprintf("Error: %v", err)})
		if encErr != nil {
			klog.Errorf("Error encoding response: %v", encErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	rawResult, err := result.Raw()
	if err != nil {
		// Send an error response
		w.WriteHeader(http.StatusInternalServerError)
		encErr := json.NewEncoder(w).Encode(healthResponse{Status: fmt.Sprintf("Error reading response: %v", err)})
		if encErr != nil {
			klog.Errorf("Error encoding response: %v", encErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// Check the health status
	if string(rawResult) == "ok" {
		// Send a healthy response
		w.WriteHeader(http.StatusOK)
		encErr := json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
		if encErr != nil {
			klog.Errorf("Error encoding response: %v", encErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		// Send an unhealthy response
		w.WriteHeader(http.StatusServiceUnavailable)
		encErr := json.NewEncoder(w).Encode(healthResponse{Status: fmt.Sprintf("unhealthy: %v", string(rawResult))})
		if encErr != nil {
			klog.Errorf("Error encoding response: %v", encErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}
