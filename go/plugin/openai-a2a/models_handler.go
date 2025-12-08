package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/agentic-layer/agent-gateway-krakend/lib/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	agentAPIVersion = "runtime.agentic-layer.ai/v1alpha1"
	agentKind       = "Agent"

	// Exponential backoff configuration
	initialBackoffSeconds = 5
	maxBackoffSeconds     = 120
	backoffMultiplier     = 2.0
)

// Agent represents a simplified Agent CRD
type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AgentSpec   `json:"spec,omitempty"`
	Status            AgentStatus `json:"status,omitempty"`
}

// AgentSpec defines the desired state of Agent
type AgentSpec struct {
	Exposed bool `json:"exposed,omitempty"`
}

// AgentStatus defines the observed state of Agent
type AgentStatus struct {
	URL string `json:"url,omitempty"`
}

// AgentList contains a list of Agents
type AgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Agent `json:"items"`
}

// DeepCopyObject implements runtime.Object interface
func (a *Agent) DeepCopyObject() runtime.Object {
	if a == nil {
		return nil
	}
	return &Agent{
		TypeMeta:   a.TypeMeta,
		ObjectMeta: *a.ObjectMeta.DeepCopy(),
		Spec: AgentSpec{
			Exposed: a.Spec.Exposed,
		},
		Status: AgentStatus{
			URL: a.Status.URL,
		},
	}
}

// DeepCopyObject implements runtime.Object interface
func (a *AgentList) DeepCopyObject() runtime.Object {
	if a == nil {
		return nil
	}
	out := &AgentList{
		TypeMeta: a.TypeMeta,
		ListMeta: *a.ListMeta.DeepCopy(),
	}
	if a.Items != nil {
		out.Items = make([]Agent, len(a.Items))
		for i := range a.Items {
			out.Items[i] = *a.Items[i].DeepCopyObject().(*Agent)
		}
	}
	return out
}

// OpenAIModel represents a model in the OpenAI API format
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// OpenAIModelsResponse represents the response for /models endpoint
type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// K8sClient wraps the Kubernetes REST client for Agent CRDs
type K8sClient struct {
	restClient rest.Interface
	scheme     *runtime.Scheme
}

var (
	// AgentGV is the GroupVersion for Agent CRDs
	AgentGV = schema.GroupVersion{
		Group:   "runtime.agentic-layer.ai",
		Version: "v1alpha1",
	}
)

var (
	// Cached K8s client instance
	cachedK8sClient   *K8sClient
	clientInitialized bool
	clientInitMutex   sync.Mutex
)

const (
	// Default TTL for agent cache (10 seconds)
	defaultAgentCacheTTL = 10 * time.Second
)

// AgentCache provides TTL-based caching for agent lists
type AgentCache struct {
	agents     []Agent
	lastUpdate time.Time
	ttl        time.Duration
	mu         sync.RWMutex
}

var (
	// Global agent cache instance
	agentCache = &AgentCache{
		ttl: defaultAgentCacheTTL,
	}
)

// BackoffTracker tracks retry attempts and calculates exponential backoff delays
type BackoffTracker struct {
	failures      int
	lastFailureAt time.Time
	mu            sync.Mutex
}

var (
	// Global backoff tracker for K8s client failures
	k8sClientBackoff = &BackoffTracker{}
)

// RecordFailure records a failed attempt and returns the backoff duration and attempt number
func (b *BackoffTracker) RecordFailure() (backoffSeconds int, attemptNumber int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures++
	b.lastFailureAt = time.Now()

	// Calculate exponential backoff: initialBackoff * (multiplier ^ (failures - 1))
	backoffSeconds = initialBackoffSeconds
	for i := 0; i < b.failures-1; i++ {
		backoffSeconds = int(float64(backoffSeconds) * backoffMultiplier)
		if backoffSeconds > maxBackoffSeconds {
			backoffSeconds = maxBackoffSeconds
			break
		}
	}

	return backoffSeconds, b.failures
}

// RecordSuccess resets the failure counter
func (b *BackoffTracker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures = 0
	b.lastFailureAt = time.Time{}
}

// Get retrieves cached agents if available and not expired, otherwise fetches fresh data
func (c *AgentCache) Get(ctx context.Context, k8sClient *K8sClient) ([]Agent, error) {
	// Try to read from cache first
	c.mu.RLock()
	if time.Since(c.lastUpdate) < c.ttl && c.agents != nil {
		// Return deep copies to prevent race conditions on ObjectMeta pointer fields
		agentsCopy := make([]Agent, len(c.agents))
		for i := range c.agents {
			agentsCopy[i] = *c.agents[i].DeepCopyObject().(*Agent)
		}
		c.mu.RUnlock()
		return agentsCopy, nil
	}
	c.mu.RUnlock()

	// Cache expired or empty, fetch fresh data
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have updated)
	if time.Since(c.lastUpdate) < c.ttl && c.agents != nil {
		// Return deep copies even under write lock for consistency
		agentsCopy := make([]Agent, len(c.agents))
		for i := range c.agents {
			agentsCopy[i] = *c.agents[i].DeepCopyObject().(*Agent)
		}
		return agentsCopy, nil
	}

	// Fetch fresh agent list from K8s
	agents, err := k8sClient.ListExposedAgents(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache
	c.agents = agents
	c.lastUpdate = time.Now()

	return agents, nil
}

// Invalidate clears the cache, forcing a fresh fetch on next Get
func (c *AgentCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agents = nil
	c.lastUpdate = time.Time{}
}

// NewK8sClient returns a cached Kubernetes client for Agent CRDs.
// The client is initialized once on first successful call and reused for subsequent calls.
// If initialization fails, it will retry on the next call.
func NewK8sClient() (*K8sClient, error) {
	clientInitMutex.Lock()
	defer clientInitMutex.Unlock()

	// Return cached client if already initialized
	if clientInitialized && cachedK8sClient != nil {
		return cachedK8sClient, nil
	}

	// Attempt to create client
	client, err := createK8sClient()
	if err != nil {
		// Don't cache the error - allow retry on next call
		return nil, err
	}

	// Cache successful initialization
	cachedK8sClient = client
	clientInitialized = true

	return cachedK8sClient, nil
}

// createK8sClient creates a new Kubernetes client for Agent CRDs.
// This function is called by NewK8sClient and will retry on failure.
func createK8sClient() (*K8sClient, error) {
	config, err := getK8sConfig()
	if err != nil {
		return nil, err
	}

	// Create a new scheme and register our types
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(AgentGV, &Agent{}, &AgentList{})
	metav1.AddToGroupVersion(scheme, AgentGV)

	// Set up API group version for Agent CRDs
	config.APIPath = "/apis"
	config.GroupVersion = &AgentGV
	config.NegotiatedSerializer = serializer.NewCodecFactory(scheme).WithoutConversion()

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}

	return &K8sClient{
		restClient: restClient,
		scheme:     scheme,
	}, nil
}

// getK8sConfig returns the Kubernetes REST configuration, trying in-cluster config first, then falling back to kubeconfig.
func getK8sConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig for local development
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		kubeconfig = filepath.Join(homeDir, ".kube", "config")
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// ListExposedAgents lists all agents with exposed: true across all namespaces
func (c *K8sClient) ListExposedAgents(ctx context.Context) ([]Agent, error) {
	result := &AgentList{}
	parameterCodec := runtime.NewParameterCodec(c.scheme)

	err := c.restClient.
		Get().
		Resource("agents").
		VersionedParams(&metav1.ListOptions{}, parameterCodec).
		Do(ctx).
		Into(result)

	if err != nil {
		return nil, err
	}

	// Filter for exposed agents
	exposedAgents := make([]Agent, 0)
	for _, agent := range result.Items {
		if agent.Spec.Exposed {
			exposedAgents = append(exposedAgents, agent)
		}
	}

	return exposedAgents, nil
}

// handleModelsRequest handles GET /models requests by listing exposed agents and returning them in OpenAI-compatible format.
func handleModelsRequest(w http.ResponseWriter, req *http.Request) {
	reqLogger := logging.NewWithPluginName(pluginName)

	if req.Method != http.MethodGet {
		reqLogger.Debug("invalid method for /models: %s", req.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reqLogger.Debug("handling /models request")

	// Create Kubernetes client
	k8sClient, err := NewK8sClient()
	if err != nil {
		backoffSeconds, attemptNumber := k8sClientBackoff.RecordFailure()
		reqLogger.Error("failed to create Kubernetes client (attempt %d): %s", attemptNumber, err)
		w.Header().Set("Retry-After", fmt.Sprintf("%d", backoffSeconds))
		http.Error(w, "service temporarily unavailable: unable to connect to Kubernetes API", http.StatusServiceUnavailable)
		return
	}

	// Record successful client creation
	k8sClientBackoff.RecordSuccess()

	// Get agents from cache (or fetch if expired)
	agents, err := agentCache.Get(req.Context(), k8sClient)
	if err != nil {
		reqLogger.Error("failed to list agents: %s", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	reqLogger.Debug("retrieved %d exposed agents", len(agents))

	// Detect name collisions
	nameCount := make(map[string]int)
	for _, agent := range agents {
		nameCount[agent.Name]++
	}

	// Build OpenAI models response
	models := make([]OpenAIModel, 0, len(agents))
	for _, agent := range agents {
		var modelID string
		if nameCount[agent.Name] > 1 {
			// Use namespaced format for collisions
			modelID = agent.Namespace + "/" + agent.Name
		} else {
			// Use simple format for unique names
			modelID = agent.Name
		}

		models = append(models, OpenAIModel{
			ID:      modelID,
			Object:  "model",
			Created: agent.CreationTimestamp.Unix(),
			OwnedBy: agent.Namespace,
		})
	}

	response := OpenAIModelsResponse{
		Object: "list",
		Data:   models,
	}

	// Marshal and send response
	responseBody, err := json.Marshal(response)
	if err != nil {
		reqLogger.Error("failed to marshal response: %s", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	reqLogger.Debug("returning %d models", len(models))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(responseBody); err != nil {
		reqLogger.Error("failed to write response: %s", err)
	}
}
