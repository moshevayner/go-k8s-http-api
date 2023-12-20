---
authors: Moshe Vayner (@moshevayner - moshe@vayner.me)
state: stable
---

# RFD 1 - HTTP API Interface for Kubernetes

## What

- Implement a new HTTP API interface for Kubernetes which would act as a proxy
  for the Kubernetes API server.

## Why

- The goal is to provide a more controlled interface for Kubernetes API, and avoid exposing the k8s API server directly to the outside world.
- This interface will provide (during preliminary phase) a subset of the Kubernetes API, and will be extended in the future to support more API resources as needed.

## Details

This interface will be implemented as a new HTTP API server, which will act as a proxy for the Kubernetes API server.
It will be written in Go, and will be deployed as a Kubernetes pod.
The interaction with k8s will be done using the official k8s go client.

From the kubernetes authentication perspective, when running the API outside of the k8s cluster, we'll use the `kubeconfig` file. By default, it'll go to `~/.kube/config`, but it can be overridden by passing `--kubeconfig` arg to the `go` command / binary.  

When running the API inside the k8s cluster, we'll use the [in-cluster config](https://pkg.go.dev/k8s.io/client-go/rest#InClusterConfig) to apply best practices.  
An initial RBAC implementation which would enable the current set of desired command for this API can be seen below:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: k8s-api-proxy
  namespace: k8s-api-proxy
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8s-api-proxy
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "update"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8s-api-proxy
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: k8s-api-proxy
subjects:
  - kind: ServiceAccount
    name: k8s-api-proxy
    namespace: k8s-api-proxy
```

The server will also have a basic in-memory cache for the k8s API responses, which will be used to reduce the load on the k8s API server (in the future, we may consider using a more advanced caching mechanism, such as Redis, but for phase 1, basic in-memory cache using the [controller-runtime Cache](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/cache) package should suffice. Using this library would also come with the advantage of built-in `EventHandlers` for automatic updates of cached resources).

### Structure / Endpoints

For phase 1, the API will have the following endpoints:

---
**Purpose:** Health Check to verify connectivity to the k8s API server. This endpoint essentially proxies the `/healthz` endpoint of the k8s API server, with the addition of a `status` field in the response body, which will be `ok` if the API server is healthy.  
**Method:** `GET`  
**Path:** `/healthz`  
**Example Response:**

```json
{
  "status": "ok",
}
```

---
**Purpose:** List available deployments in the cluster (and if specified- in the given namespace)
**Method:** `GET`  
**Path:** `/deployments?namespace={namespace}`  
**Query Params:**

- `namespace` (optional). If not specified, will return all deployments in the cluster. If specified, will return all deployments in the given namespace.

**Example Response:**

```json
[
  {
    "name": "foo",
    "namespace": "default",
  },
  {
    "name": "bar",
    "namespace": "baz",
  }
]
```

---
**Purpose:** Get the number of replicas for a given deployment  
**Method:** `GET`  
**Path:** `/deployments/{namespace}/{deployment}/replicas`  
**Example Response:**

```json
{
  "deployment": "foo",
  "namespace": "default",
  "replicas": 3
}
```

---
**Purpose:** Set the number of replicas for a given deployment  
**Method:** `PUT`  
**Path:** `/deployments/{namespace}/{deployment}/replicas`  
**Body:**

```json
{
  "replicas": 3
}
```

**Example Response:**

```json
{
  "deployment": "foo",
  "namespace": "default",
  "replicas": 3
}
```

---

### Security

The API server will be available via HTTPS only, using mTLS authentication.
For phase one, we will use a self-signed certificate, and in the future, we will consider using a proper CA such as CertManager with LetsEncrypt.

## Build / Deploy / Test

### Prerequisites

At the very least, you will need the following tools installed on your machine, in order to use the `Makefile` targets that are described in the next section:

1. [Docker](https://docs.docker.com/get-docker/)
1. Access to a Kubernetes cluster (either local or remote). Examples could be [Minikube](https://minikube.sigs.k8s.io/docs/start/), [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/), or a remote cluster such as [GKE](https://cloud.google.com/kubernetes-engine/docs/quickstart).
1. Kubernetes CLI ([kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) configured to access the cluster)

### Primary `Makefile` Targets

### `build`

Build the API server binary. Will be stored in the `bin` directory as `api`.

### `run`

Run the API server locally. This will use the `kubeconfig` file that is stored in the `~/.kube/config` directory by default (can be overridden through the `$KUBECONFIG` variable). Make sure to follow the instructions in the `config/README.md` file for more details.

### `generate-certs`

Generate the self-signed set of certificates for the API server (CA, server, client). The certificates will be stored in a local directory, from which the Helm chart will read them. **IMPORTANT**: Make sure NOT to modify that path for two reasons:

1. The go binary (when running locally) will look for the certificates in that path if using the `Makefile` target `run`
1. The `.gitignore` file will ignore that path, so that the certificates will not be committed to the repository

### `docker-build-push`

Build the docker image for the API server.  

**NOTE**: `IMG` environment variable should be set for this target, i.e. `registry/repo:tag`.

### `deploy`

Deploy the Helm chart to the cluster. Make sure to follow the Helm Chart's `values.yaml` file for the configuration. Most importantly- populate the base64 encoded certificates and keys in the `values.yaml` file.

### `undeploy`

Uninstall the Helm chart from the cluster including the release name and namespace.

### `test`

Run the unit tests

### `ci`

Run the CI tests (unit tests, linting, etc.)

## Accessing the API

### Networking / Ingress

From a networking standpoint, you can either use an Ingress Controller (Ingress Resource can be generated by the Helm chart, see `ingress` stanza in the `values.yaml` for more details), or use `kubectl port-forward` to forward the API server port to your local machine. You can  follow [these instructions](https://kubernetes.io/docs/tasks/access-application-cluster/port-forward-access-application-cluster/#forward-a-local-port-to-a-port-on-the-pod) for more info.
You may also see the below example for a quick start. This will forward the API server port to your local machine on port 8443:

```bash
kubectl -n <namespace> port-forward svc/k8s-api-proxy 8443:443
```

### Basic Usage

From a basic usage standpoint, the quickest way to get started is to use `curl` to access the API. Make sure to use the pre-generated certs for in your command, since the API is secured using `mTLS` and requires authentication. For example (assuming you are running `kubectl port-forward` to forward the API server port to your local machine. If you are using a public endpoint, make sure to replace the server address in the command below):

```bash
curl --cacert ./certs/ca.crt --cert ./certs/client.crt --key ./certs/client.key https://localhost:8443/deployments
```
