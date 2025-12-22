package collectors

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
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
		errorReason := classifyK8sError(err)
		log.Printf("Kubernetes API Server is unreachable: %v [reason: %s]", err, errorReason)
		metrics.KubernetesAPIServerUp.WithLabelValues(errorReason).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("kubernetes_apiserver", "unreachable").Inc()
		return
	}

	log.Printf("Kubernetes API Server is healthy")
	metrics.KubernetesAPIServerUp.WithLabelValues("正常").Set(1)
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
		errorReason := classifyK8sError(err)
		log.Printf("Failed to list CoreDNS pods: %v [reason: %s]", err, errorReason)
		metrics.CoreDNSUp.WithLabelValues(errorReason).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("coredns", "list_failed").Inc()
		return
	}

	if len(pods.Items) == 0 {
		log.Printf("No CoreDNS pods found")
		metrics.CoreDNSUp.WithLabelValues("未找到Pod").Set(0)
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
		metrics.CoreDNSUp.WithLabelValues("无就绪Pod").Set(0)
		metrics.HealthCheckErrors.WithLabelValues("coredns", "no_ready_pods").Inc()
		return
	}

	// Perform DNS resolution test
	_, err = net.LookupHost("kubernetes.default.svc.cluster.local")
	if err != nil {
		errorReason := classifyDNSError(err)
		log.Printf("DNS resolution test failed: %v [reason: %s]", err, errorReason)
		metrics.CoreDNSUp.WithLabelValues(errorReason).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("coredns", "resolution_failed").Inc()
		return
	}

	log.Printf("CoreDNS is healthy")
	metrics.CoreDNSUp.WithLabelValues("正常").Set(1)
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
		errorReason := classifyK8sError(err)
		log.Printf("Failed to list etcd pods: %v [reason: %s]", err, errorReason)
		metrics.EtcdUp.WithLabelValues(errorReason).Set(0)
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
			errorReason := classifyK8sError(err)
			log.Printf("Etcd health check via API Server failed: %v [reason: %s]", err, errorReason)
			metrics.EtcdUp.WithLabelValues(errorReason).Set(0)
			metrics.HealthCheckErrors.WithLabelValues("etcd", "health_check_failed").Inc()
			return
		}

		log.Printf("Etcd is healthy (verified via API Server)")
		metrics.EtcdUp.WithLabelValues("正常").Set(1)
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
		metrics.EtcdUp.WithLabelValues("无运行中Pod").Set(0)
		metrics.HealthCheckErrors.WithLabelValues("etcd", "no_running_pods").Inc()
		return
	}

	log.Printf("Etcd is healthy")
	metrics.EtcdUp.WithLabelValues("正常").Set(1)
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
		errorReason := classifyK8sError(err)
		log.Printf("Failed to list storage classes: %v [reason: %s]", err, errorReason)
		metrics.ClusterStorageUp.WithLabelValues("default", errorReason).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("storage_class", "list_failed").Inc()
		return
	}

	if len(storageClasses.Items) == 0 {
		log.Printf("No storage classes found")
		metrics.ClusterStorageUp.WithLabelValues("default", "未找到存储类").Set(0)
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

	// Get storage class to check binding mode
	sc, err := c.clientset.StorageV1().StorageClasses().Get(ctx, storageClassName, metav1.GetOptions{})
	if err != nil {
		errorReason := classifyK8sError(err)
		log.Printf("Failed to get storage class %s: %v [reason: %s]", storageClassName, err, errorReason)
		metrics.ClusterStorageUp.WithLabelValues(storageClassName, errorReason).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("storage_class", "get_failed").Inc()
		return
	}

	// Check if it's WaitForFirstConsumer binding mode
	isWaitForFirstConsumer := sc.VolumeBindingMode != nil && *sc.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer

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

	if isWaitForFirstConsumer {
		log.Printf("Testing storage class %s (WaitForFirstConsumer mode) by creating test PVC %s...", storageClassName, testPVCName)
	} else {
		log.Printf("Testing storage class %s by creating test PVC %s...", storageClassName, testPVCName)
	}

	// Create the test PVC
	_, err = c.clientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		errorReason := classifyK8sError(err)
		log.Printf("Failed to create test PVC for storage class %s: %v [reason: %s]", storageClassName, err, errorReason)
		metrics.ClusterStorageUp.WithLabelValues(storageClassName, errorReason).Set(0)
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

	// For WaitForFirstConsumer storage classes, just verify PVC was created successfully
	// and is in Pending state (waiting for a Pod to consume it)
	if isWaitForFirstConsumer {
		// Wait a bit to ensure PVC status is updated
		time.Sleep(2 * time.Second)

		currentPVC, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, testPVCName, metav1.GetOptions{})
		if err != nil {
			errorReason := classifyK8sError(err)
			log.Printf("Failed to get test PVC %s status: %v [reason: %s]", testPVCName, err, errorReason)
			metrics.ClusterStorageUp.WithLabelValues(storageClassName, errorReason).Set(0)
			metrics.HealthCheckErrors.WithLabelValues("storage_class", "pvc_get_failed").Inc()
			return
		}

		// For WaitForFirstConsumer, Pending is the expected state
		if currentPVC.Status.Phase == corev1.ClaimPending {
			log.Printf("Test PVC %s is Pending (WaitForFirstConsumer), storage class %s is functional", testPVCName, storageClassName)
			metrics.ClusterStorageUp.WithLabelValues(storageClassName, "正常").Set(1)
			return
		}

		// If it somehow got bound (rare but possible), that's also good
		if currentPVC.Status.Phase == corev1.ClaimBound {
			log.Printf("Test PVC %s is Bound, storage class %s is functional", testPVCName, storageClassName)
			metrics.ClusterStorageUp.WithLabelValues(storageClassName, "正常").Set(1)
			return
		}

		// Any other state is problematic
		errorReason := fmt.Sprintf("PVC状态异常_%s", string(currentPVC.Status.Phase))
		log.Printf("Test PVC %s in unexpected state %s for WaitForFirstConsumer storage class", testPVCName, currentPVC.Status.Phase)
		metrics.ClusterStorageUp.WithLabelValues(storageClassName, errorReason).Set(0)
		metrics.HealthCheckErrors.WithLabelValues("storage_class", "unexpected_state").Inc()
		return
	}

	// For Immediate binding mode, wait for PVC to become Bound
	log.Printf("Waiting for test PVC %s to become Bound...", testPVCName)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-timeout:
			log.Printf("Timeout waiting for test PVC %s to bind (storage class %s may be slow or unavailable)", testPVCName, storageClassName)
			metrics.ClusterStorageUp.WithLabelValues(storageClassName, "PVC绑定超时").Set(0)
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
				metrics.ClusterStorageUp.WithLabelValues(storageClassName, "正常").Set(1)
				return
			}

			// Check if PVC is in a failed state
			if currentPVC.Status.Phase == corev1.ClaimLost {
				log.Printf("Test PVC %s is in Lost state, storage class %s may have issues", testPVCName, storageClassName)
				metrics.ClusterStorageUp.WithLabelValues(storageClassName, "PVC丢失").Set(0)
				metrics.HealthCheckErrors.WithLabelValues("storage_class", "pvc_lost").Inc()
				return
			}

			// Log current status
			log.Printf("Test PVC %s status: %s (waiting for Bound)", testPVCName, currentPVC.Status.Phase)

		case <-ctx.Done():
			log.Printf("Context cancelled while testing storage class %s", storageClassName)
			metrics.ClusterStorageUp.WithLabelValues(storageClassName, "上下文已取消").Set(0)
			metrics.HealthCheckErrors.WithLabelValues("storage_class", "context_cancelled").Inc()
			return
		}
	}
}

// classifyK8sError classifies Kubernetes API errors for better troubleshooting
func classifyK8sError(err error) string {
	if err == nil {
		return "正常"
	}

	errMsg := strings.ToLower(err.Error())

	// Authentication/Authorization errors
	if strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "authentication") {
		return "认证失败"
	}
	if strings.Contains(errMsg, "forbidden") || strings.Contains(errMsg, "authorization") {
		return "授权失败"
	}

	// Connection timeout
	if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded") {
		return "连接超时"
	}

	// Network errors
	if strings.Contains(errMsg, "connection refused") {
		return "连接被拒绝"
	}
	if strings.Contains(errMsg, "no route to host") || strings.Contains(errMsg, "network unreachable") {
		return "网络不可达"
	}
	if strings.Contains(errMsg, "connection reset") {
		return "连接被重置"
	}

	// DNS resolution errors
	if strings.Contains(errMsg, "no such host") || strings.Contains(errMsg, "could not resolve") {
		return "DNS解析失败"
	}

	// TLS/Certificate errors
	if strings.Contains(errMsg, "certificate") || strings.Contains(errMsg, "tls") || strings.Contains(errMsg, "x509") {
		return "TLS证书错误"
	}

	// Resource not found
	if strings.Contains(errMsg, "not found") {
		return "资源不存在"
	}

	// API server unavailable
	if strings.Contains(errMsg, "server could not find") || strings.Contains(errMsg, "the server is currently unable") {
		return "API服务不可用"
	}

	// Generic connection error
	if strings.Contains(errMsg, "connection") {
		return "连接错误"
	}

	// Unknown error
	return "未知错误"
}

// classifyDNSError classifies DNS resolution errors for better troubleshooting
func classifyDNSError(err error) string {
	if err == nil {
		return "正常"
	}

	errMsg := strings.ToLower(err.Error())

	// DNS resolution failed
	if strings.Contains(errMsg, "no such host") {
		return "DNS主机不存在"
	}
	if strings.Contains(errMsg, "server misbehaving") {
		return "DNS服务器错误"
	}
	if strings.Contains(errMsg, "timeout") {
		return "DNS超时"
	}
	if strings.Contains(errMsg, "temporary failure") {
		return "DNS临时故障"
	}

	// Generic DNS error
	return "DNS解析失败"
}
