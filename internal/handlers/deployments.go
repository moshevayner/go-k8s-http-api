package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploymentResponse is the response object for the deployments API
type DeploymentResponse struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// Replicas is the request / response object for the deployments API
type Replicas struct {
	Replicas *int32 `json:"replicas"`
}

// APIError is a response object for cases when an error occurs
type APIError struct {
	Message string `json:"message"`
}

// Validate validates the Replicas object and returns an error if it is invalid
func (r *Replicas) Validate() error {
	if r.Replicas == nil {
		return fmt.Errorf("replicas field is required")
	} else if *r.Replicas < 0 {
		return fmt.Errorf("replicas field must be greater than or equal to 0")
	}

	return nil
}

// DeploymentResponseWithReplicas is the response object for the deployments API
type DeploymentResponseWithReplicas struct {
	DeploymentResponse
	Replicas
}

// DeploymentsHandler is the handler for the deployments API
type DeploymentsHandler struct {
	client.Client
}

// ListDeployments handles the "/deployments" endpoint
func (h *DeploymentsHandler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	dl := &appsv1.DeploymentList{}

	// If namespace was passed as a query parameter, use it. Otherwise return deployments from all namespaces.
	if namespace := r.URL.Query().Get("namespace"); namespace != "" {
		err := h.List(r.Context(), dl, client.InNamespace(namespace))
		if err != nil {
			klog.Errorf("Error listing deployments in namespace %s: %v", namespace, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		// list deployments in all namespaces
		err := h.List(r.Context(), dl)
		if err != nil {
			klog.Errorf("Error listing deployments in all namespaces: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	response := generateListDeploymentsResponse(dl)
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		klog.Errorf("Error encoding response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// GetDeploymentReplicas handles the "/deployments/{namespace}/{deployment}/replicas" endpoint for GET method
func (h *DeploymentsHandler) GetDeploymentReplicas(w http.ResponseWriter, r *http.Request) {
	// Parse namespace and deployment from the URL path
	namespace, deployment := parseNamespaceAndDeploymentNameFromURL(r)

	// Get the deployment object
	d, err := h.getDeployment(r.Context(), namespace, deployment)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		encErr := json.NewEncoder(w).Encode(APIError{fmt.Sprintf("Error getting deployment %s in namespace %s", deployment, namespace)})
		if encErr != nil {
			klog.Errorf("Error encoding response: %v", encErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	// Get the deployment's replicas field
	replicas := d.Spec.Replicas
	// Return the replicas field as a JSON response
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(DeploymentResponseWithReplicas{
		DeploymentResponse: DeploymentResponse{
			Name:      deployment,
			Namespace: namespace,
		},
		Replicas: Replicas{replicas},
	},
	)
	if err != nil {
		klog.Errorf("Error encoding response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// SetDeploymentReplicas handles the "/deployments/{namespace}/{deployment}/replicas" endpoint for PUT method
func (h *DeploymentsHandler) SetDeploymentReplicas(w http.ResponseWriter, r *http.Request) {
	namespace, deployment := parseNamespaceAndDeploymentNameFromURL(r)

	// Get the deployment object
	d, err := h.getDeployment(r.Context(), namespace, deployment)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		encErr := json.NewEncoder(w).Encode(APIError{fmt.Sprintf("Error getting deployment %s in namespace %s", deployment, namespace)})
		if encErr != nil {
			klog.Errorf("Error encoding response: %v", encErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// Parse the request body
	var rep Replicas
	err = json.NewDecoder(r.Body).Decode(&rep)
	if err != nil {
		// log the error, return a 400 Bad Request and the error message
		resp := fmt.Sprintf("Error parsing request body: %v", err)
		klog.Errorf("%v", resp)
		w.WriteHeader(http.StatusBadRequest)
		encErr := json.NewEncoder(w).Encode(APIError{resp})
		if encErr != nil {
			klog.Errorf("Error encoding response: %v", encErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// validate the passed in replicas
	err = rep.Validate()
	if err != nil {
		resp := fmt.Sprintf("Validation error: %v", err)
		klog.Errorf("%v", resp)
		w.WriteHeader(http.StatusBadRequest)
		encErr := json.NewEncoder(w).Encode(APIError{resp})
		if encErr != nil {
			klog.Errorf("Error encoding response: %v", encErr)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// Create a patch that updates the replicas field
	patch := client.MergeFrom(d.DeepCopy())
	d.Spec.Replicas = rep.Replicas
	err = h.Patch(r.Context(), d, patch)
	if err != nil {
		klog.Errorf("Error patching deployment %s in namespace %s: %v", deployment, namespace, err)
		w.WriteHeader(http.StatusInternalServerError)
		encErr := json.NewEncoder(w).Encode(APIError{fmt.Sprintf("Error patching deployment %s in namespace %s", deployment, namespace)})
		if encErr != nil {
			klog.Errorf("Error encoding response: %v", encErr)
		}
		return
	}

	// Return the replicas field as a JSON response
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(DeploymentResponseWithReplicas{
		DeploymentResponse: DeploymentResponse{
			Name:      deployment,
			Namespace: namespace,
		},
		Replicas: Replicas{d.Spec.Replicas},
	},
	)
	if err != nil {
		klog.Errorf("Error encoding response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// generateListDeploymentsResponse generates a list of DeploymentResponse objects from a DeploymentList
func generateListDeploymentsResponse(result *appsv1.DeploymentList) []DeploymentResponse {
	response := make([]DeploymentResponse, 0, len(result.Items))
	for _, d := range result.Items {
		klog.V(5).Infof("Deployment %s in namespace %s", d.Name, d.Namespace)
		response = append(response, DeploymentResponse{
			Name:      d.Name,
			Namespace: d.Namespace,
		})
	}
	return response
}

// parseNamespaceAndDeploymentNameFromURL parses the namespace and deployment name from the URL path
func parseNamespaceAndDeploymentNameFromURL(r *http.Request) (string, string) {
	// Split the URL path into segments
	pathSegments := strings.Split(r.URL.Path, "/")

	// Check if the pathSegments slice has at least 4 elements. If not- return empty strings for now
	// TODO we may want to return an error here instead in a future iteration
	if len(pathSegments) < 4 {
		klog.Errorf("Error parsing namespace and deployment name from URL path: %s", r.URL.Path)
		return "", ""
	}

	// Extract namespace and deployment name from the URL path
	namespace := pathSegments[2]
	deployment := pathSegments[3]

	return namespace, deployment
}

// getDeployment returns a deployment object from the client (either from the cache or from the API)
func (h *DeploymentsHandler) getDeployment(ctx context.Context, namespace, deployment string) (*appsv1.Deployment, error) {
	d := &appsv1.Deployment{}
	err := h.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      deployment,
	}, d)
	if err != nil {
		klog.Errorf("Error getting deployment %s in namespace %s: %v", deployment, namespace, err)
		return nil, err
	}
	return d, nil
}
