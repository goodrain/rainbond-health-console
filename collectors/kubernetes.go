package collectors

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/rainbond/health-console/config"
	"github.com/rainbond/health-console/metrics"
)

// KubernetesCollector monitors Kubernetes cluster health
type KubernetesCollector struct {
	clientset *kubernetes.Clientset
	interval  time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewKubernetesCollector creates a new Kubernetes collector
func NewKubernetesCollector(cfg *config.Config) (*KubernetesCollector, error) {
	// Create in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &KubernetesCollector{
		clientset: clientset,
		interval:  cfg.CollectInterval,
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

// Start begins collecting Kubernetes metrics
func (c *KubernetesCollector) Start() {
	log.Println("Starting Kubernetes collector...")

	// Initial check
	c.collect()

	// Periodic checks
	ticker := time.NewTicker(c.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				c.collect()
			case <-c.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops the collector
func (c *KubernetesCollector) Stop() {
	log.Println("Stopping Kubernetes collector...")
	c.cancel()
}

// collect performs all Kubernetes health checks
func (c *KubernetesCollector) collect() {
	go c.checkAPIServer()
	go c.checkCoreDNS()
	go c.checkEtcd()
	go c.checkStorageClasses()
}

// checkAPIServer checks if API Server is reachable
func (c *KubernetesCollector) checkAPIServer() {
	start := time.Now()
	defer func() {
		metrics.HealthCheckDuration.WithLabelValues("kubernetes_apiserver").Observe(time.Since(start).Seconds())
	}()

	// Try to get server version
	_, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		log.Printf("Kubernetes API Server is unreachable: %v", err)
		metrics.KubernetesAPIServerUp.Set(0)
		metrics.HealthCheckErrors.WithLabelValues("kubernetes_apiserver", "unreachable").Inc()
		return
	}

	log.Printf("Kubernetes API Server is healthy")
	metrics.KubernetesAPIServerUp.Set(1)
}

// checkCoreDNS checks if CoreDNS is working properly
func (c *KubernetesCollector) checkCoreDNS() {
	start := time.Now()
	defer func() {
		metrics.HealthCheckDuration.WithLabelValues("coredns").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check CoreDNS pods in kube-system namespace
	pods, err := c.clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=kube-dns",
	})
	if err != nil {
		log.Printf("Failed to list CoreDNS pods: %v", err)
		metrics.CoreDNSUp.Set(0)
		metrics.HealthCheckErrors.WithLabelValues("coredns", "list_failed").Inc()
		return
	}

	if len(pods.Items) == 0 {
		log.Printf("No CoreDNS pods found")
		metrics.CoreDNSUp.Set(0)
		metrics.HealthCheckErrors.WithLabelValues("coredns", "no_pods").Inc()
		return
	}

	// Check if at least one CoreDNS pod is running and ready
	hasReadyPod := false
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					hasReadyPod = true
					break
				}
			}
		}
		if hasReadyPod {
			break
		}
	}

	if !hasReadyPod {
		log.Printf("No ready CoreDNS pods found")
		metrics.CoreDNSUp.Set(0)
		metrics.HealthCheckErrors.WithLabelValues("coredns", "no_ready_pods").Inc()
		return
	}

	// Perform DNS resolution test
	_, err = net.LookupHost("kubernetes.default.svc.cluster.local")
	if err != nil {
		log.Printf("DNS resolution test failed: %v", err)
		metrics.CoreDNSUp.Set(0)
		metrics.HealthCheckErrors.WithLabelValues("coredns", "resolution_failed").Inc()
		return
	}

	log.Printf("CoreDNS is healthy")
	metrics.CoreDNSUp.Set(1)
}

// checkEtcd checks if Etcd cluster is available
func (c *KubernetesCollector) checkEtcd() {
	start := time.Now()
	defer func() {
		metrics.HealthCheckDuration.WithLabelValues("etcd").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check etcd pods in kube-system namespace
	// In some clusters, etcd might be running as static pods or outside the cluster
	pods, err := c.clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "component=etcd",
	})
	if err != nil {
		log.Printf("Failed to list etcd pods: %v", err)
		metrics.EtcdUp.Set(0)
		metrics.HealthCheckErrors.WithLabelValues("etcd", "list_failed").Inc()
		return
	}

	// If no etcd pods found, try to check via API Server health
	// (API Server depends on etcd, so if API Server is up, etcd is likely up)
	if len(pods.Items) == 0 {
		// Check API Server livez endpoint which includes etcd check
		req := c.clientset.Discovery().RESTClient().Get().AbsPath("/livez")
		result := req.Do(ctx)
		if err := result.Error(); err != nil {
			log.Printf("Etcd health check via API Server failed: %v", err)
			metrics.EtcdUp.Set(0)
			metrics.HealthCheckErrors.WithLabelValues("etcd", "health_check_failed").Inc()
			return
		}

		log.Printf("Etcd is healthy (verified via API Server)")
		metrics.EtcdUp.Set(1)
		return
	}

	// Check if at least one etcd pod is running
	hasRunningPod := false
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			hasRunningPod = true
			break
		}
	}

	if !hasRunningPod {
		log.Printf("No running etcd pods found")
		metrics.EtcdUp.Set(0)
		metrics.HealthCheckErrors.WithLabelValues("etcd", "no_running_pods").Inc()
		return
	}

	log.Printf("Etcd is healthy")
	metrics.EtcdUp.Set(1)
}

// checkStorageClasses checks if storage classes are available by creating test PVCs
func (c *KubernetesCollector) checkStorageClasses() {
	start := time.Now()
	defer func() {
		metrics.HealthCheckDuration.WithLabelValues("storage_class").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// List all storage classes
	storageClasses, err := c.clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Failed to list storage classes: %v", err)
		metrics.ClusterStorageUp.WithLabelValues("default").Set(0)
		metrics.HealthCheckErrors.WithLabelValues("storage_class", "list_failed").Inc()
		return
	}

	if len(storageClasses.Items) == 0 {
		log.Printf("No storage classes found")
		metrics.ClusterStorageUp.WithLabelValues("default").Set(0)
		metrics.HealthCheckErrors.WithLabelValues("storage_class", "no_storage_classes").Inc()
		return
	}

	// Check each storage class by creating a test PVC
	for _, sc := range storageClasses.Items {
		go c.testStorageClass(sc.Name)
	}
}

// testStorageClass tests if a storage class is functional by creating a test PVC
func (c *KubernetesCollector) testStorageClass(storageClassName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Generate unique test PVC name
	testPVCName := fmt.Sprintf("health-check-test-%s-%d", storageClassName, time.Now().Unix())
	namespace := "rbd-system" // Use rbd-system namespace for health checks

	// Create test PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPVCName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":     "health-console",
				"purpose": "storage-test",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Mi"), // Minimal size
				},
			},
		},
	}

	log.Printf("Testing storage class %s by creating test PVC %s...", storageClassName, testPVCName)

	// Create the test PVC
	_, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create test PVC for storage class %s: %v", storageClassName, err)
		metrics.ClusterStorageUp.WithLabelValues(storageClassName).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("storage_class", "pvc_create_failed").Inc()
		return
	}

	// Ensure cleanup on exit
	defer func() {
		// Delete the test PVC (the PV will be automatically deleted if dynamically provisioned)
		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer deleteCancel()

		err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(deleteCtx, testPVCName, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Warning: Failed to delete test PVC %s: %v", testPVCName, err)
		} else {
			log.Printf("Cleaned up test PVC %s for storage class %s", testPVCName, storageClassName)
		}
	}()

	// Wait for PVC to become Bound (with timeout)
	log.Printf("Waiting for test PVC %s to become Bound...", testPVCName)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-timeout:
			log.Printf("Timeout waiting for test PVC %s to bind (storage class %s may be slow or unavailable)", testPVCName, storageClassName)
			metrics.ClusterStorageUp.WithLabelValues(storageClassName).Set(0)
			metrics.HealthCheckErrors.WithLabelValues("storage_class", "pvc_bind_timeout").Inc()
			return

		case <-ticker.C:
			// Check PVC status
			currentPVC, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, testPVCName, metav1.GetOptions{})
			if err != nil {
				log.Printf("Failed to get test PVC %s status: %v", testPVCName, err)
				continue
			}

			// Check if PVC is Bound
			if currentPVC.Status.Phase == corev1.ClaimBound {
				log.Printf("Test PVC %s successfully bound! Storage class %s is functional", testPVCName, storageClassName)
				metrics.ClusterStorageUp.WithLabelValues(storageClassName).Set(1)
				return
			}

			// Check if PVC is in a failed state
			if currentPVC.Status.Phase == corev1.ClaimLost {
				log.Printf("Test PVC %s is in Lost state, storage class %s may have issues", testPVCName, storageClassName)
				metrics.ClusterStorageUp.WithLabelValues(storageClassName).Set(0)
				metrics.HealthCheckErrors.WithLabelValues("storage_class", "pvc_lost").Inc()
				return
			}

			// Log current status
			log.Printf("Test PVC %s status: %s (waiting for Bound)", testPVCName, currentPVC.Status.Phase)

		case <-ctx.Done():
			log.Printf("Context cancelled while testing storage class %s", storageClassName)
			metrics.ClusterStorageUp.WithLabelValues(storageClassName).Set(0)
			metrics.HealthCheckErrors.WithLabelValues("storage_class", "context_cancelled").Inc()
			return
		}
	}
}
