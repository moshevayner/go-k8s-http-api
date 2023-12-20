package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/utils/ptr"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// newHttpTestRequest creates a new http.Request for testing purposes
func newHttpTestRequest(method, url string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, url, body)
	return req
}

// newResponseRecorder creates a new http.ResponseWriter for testing purposes
func newResponseRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

func TestDeploymentsHandler_ListDeployments(t *testing.T) {
	testScheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(testScheme) // Register apps/v1 types
	type fields struct {
		Client client.Client
	}
	type args struct {
		w http.ResponseWriter
		r *http.Request
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		expectedResponse string
	}{
		{
			"Test ListDeployments All Namespaces",
			fields{
				Client: fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "test-namespace",
					},
				},
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-deployment2",
							Namespace: "test-namespace2",
						},
					}).Build(),
			},
			args{
				w: newResponseRecorder(),
				r: newHttpTestRequest("GET", "/deployments", nil),
			},
			"[{\"name\":\"test-deployment\",\"namespace\":\"test-namespace\"},{\"name\":\"test-deployment2\",\"namespace\":\"test-namespace2\"}]\n",
		},
		{
			"Test ListDeployments Single Namespace",
			fields{
				Client: fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "test-namespace",
					},
				},
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-deployment2",
							Namespace: "test-namespace2",
						},
					}).Build(),
			},
			args{
				w: newResponseRecorder(),
				r: newHttpTestRequest("GET", "/deployments?namespace=test-namespace", nil),
			},
			"[{\"name\":\"test-deployment\",\"namespace\":\"test-namespace\"}]\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &DeploymentsHandler{
				Client: tt.fields.Client,
			}
			h.ListDeployments(tt.args.w, tt.args.r)

			// Check the response status code
			if tt.args.w.(*httptest.ResponseRecorder).Code != http.StatusOK {
				t.Errorf("ListDeployments() status code = %v, want %v", tt.args.w.(*httptest.ResponseRecorder).Code, http.StatusOK)
			}

			// Check the response body
			rb := tt.args.w.(*httptest.ResponseRecorder).Body.String()
			if rb != tt.expectedResponse {
				t.Errorf("ListDeployments() response body = %v, want %v", rb, tt.expectedResponse)
			}
		})
	}
}

func TestDeploymentsHandler_GetDeploymentReplicas(t *testing.T) {
	testScheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(testScheme) // Register apps/v1 types
	type fields struct {
		Client client.Client
	}
	type args struct {
		w http.ResponseWriter
		r *http.Request
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		expectedStatus   int
		expectedResponse string
	}{
		{
			"Test GetDeploymentReplicas",
			fields{
				Client: fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "test-namespace",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: ptr.To(int32(3)),
					},
				}).Build(),
			},
			args{
				w: newResponseRecorder(),
				r: newHttpTestRequest("GET", "/deployments/test-namespace/test-deployment/replicas", nil),
			},
			http.StatusOK,
			"{\"name\":\"test-deployment\",\"namespace\":\"test-namespace\",\"replicas\":3}\n",
		},
		{
			"Test GetDeploymentReplicas Not Found",
			fields{
				Client: fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects().Build(),
			},
			args{
				w: newResponseRecorder(),
				r: newHttpTestRequest("GET", "/deployments/foo/bar/replicas", nil),
			},
			http.StatusNotFound,
			"{\"message\":\"Error getting deployment bar in namespace foo\"}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &DeploymentsHandler{
				Client: tt.fields.Client,
			}
			h.GetDeploymentReplicas(tt.args.w, tt.args.r)

			// Check the response status code
			if tt.args.w.(*httptest.ResponseRecorder).Code != tt.expectedStatus {
				t.Errorf("GetDeploymentReplicas() status code = %v, want %v", tt.args.w.(*httptest.ResponseRecorder).Code, tt.expectedStatus)
			}

			// Check the response body
			rb := tt.args.w.(*httptest.ResponseRecorder).Body.String()
			if rb != tt.expectedResponse {
				t.Errorf("GetDeploymentReplicas() response body = %v, want %v", rb, tt.expectedResponse)
			}
		})
	}
}

func TestDeploymentsHandler_SetDeploymentReplicas(t *testing.T) {
	testScheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(testScheme) // Register apps/v1 types
	type fields struct {
		Client client.Client
	}
	type args struct {
		w http.ResponseWriter
		r *http.Request
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		expectedStatus   int
		expectedResponse string
	}{
		{
			"Test SetDeploymentReplicas",
			fields{
				Client: fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "test-namespace",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: ptr.To(int32(3)),
					},
				}).Build(),
			},
			args{
				w: newResponseRecorder(),
				r: newHttpTestRequest("PUT", "/deployments/test-namespace/test-deployment/replicas", strings.NewReader("{\"replicas\":7}")),
			},
			http.StatusOK,
			"{\"name\":\"test-deployment\",\"namespace\":\"test-namespace\",\"replicas\":7}\n",
		},
		{
			"Test SetDeploymentReplicas Not Found",
			fields{
				Client: fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "test-namespace",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: ptr.To(int32(3)),
					},
				}).Build(),
			},
			args{
				w: newResponseRecorder(),
				r: newHttpTestRequest("PUT", "/deployments/foo/bar/replicas", strings.NewReader("{\"replicas\":99}")),
			},
			http.StatusNotFound,
			"{\"message\":\"Error getting deployment bar in namespace foo\"}\n",
		},
		{
			"Test SetDeploymentReplicas Bad Request - typo in request body",
			fields{
				Client: fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bar",
						Namespace: "foo",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: ptr.To(int32(3)),
					},
				}).Build(),
			},
			args{
				w: newResponseRecorder(),
				r: newHttpTestRequest("PUT", "/deployments/foo/bar/replicas", strings.NewReader("{\"replikas\":99}")),
			},
			http.StatusBadRequest,
			"{\"message\":\"Validation error: replicas field is required\"}\n",
		},
		{
			"Test SetDeploymentReplicas Bad Request - negative replicas",
			fields{
				Client: fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bar",
						Namespace: "foo",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: ptr.To(int32(3)),
					},
				}).Build(),
			},
			args{
				w: newResponseRecorder(),
				r: newHttpTestRequest("PUT", "/deployments/foo/bar/replicas", strings.NewReader("{\"replicas\":-1}")),
			},
			http.StatusBadRequest,
			"{\"message\":\"Validation error: replicas field must be greater than or equal to 0\"}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &DeploymentsHandler{
				Client: tt.fields.Client,
			}
			h.SetDeploymentReplicas(tt.args.w, tt.args.r)

			// Check the response status code
			if tt.args.w.(*httptest.ResponseRecorder).Code != tt.expectedStatus {
				t.Errorf("SetDeploymentReplicas() status code = %v, want %v", tt.args.w.(*httptest.ResponseRecorder).Code, tt.expectedStatus)
			}

			// Check the response body
			rb := tt.args.w.(*httptest.ResponseRecorder).Body.String()
			if rb != tt.expectedResponse {
				t.Errorf("SetDeploymentReplicas() response body = %v, want %v", rb, tt.expectedResponse)
			}
		})
	}
}
