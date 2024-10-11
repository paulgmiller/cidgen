package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumclientset "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Parse kubeconfig flag
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	count := flag.Int("count", 100, "number of identites")
	flag.Parse()

	ctx := context.Background()

	// Build Kubernetes client configuration
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// Create Cilium clientset
	clientset, err := ciliumclientset.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	for i := 0; i < *count; i++ {
		// Generate random labels and security labels
		labels := generateRandomLabels()
		securityLabels := generateRandomSecurityLabels()

		// Generate a random identity ID
		identityID := rand.Intn(100000) + 1

		// Create a new CiliumIdentity object
		ciliumIdentity := &ciliumv2.CiliumIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%d", identityID),
				Labels: labels,
			},
			SecurityLabels: securityLabels,
		}

		// Create the CiliumIdentity in Kubernetes

		_, err = clientset.CiliumV2().CiliumIdentities().Create(ctx, ciliumIdentity, metav1.CreateOptions{})
		if err != nil {
			panic(err.Error())
		}

		fmt.Printf("Created CiliumIdentity with ID: %d\n", identityID)
	}
}

// generateRandomLabels creates random labels for the CiliumIdentity metadata.
func generateRandomLabels() map[string]string {
	return map[string]string{
		"io.cilium.k8s.policy.cluster":        "default",
		"io.cilium.k8s.policy.serviceaccount": fmt.Sprintf("serviceaccount-%d", rand.Intn(1000)),
		"io.kubernetes.pod.namespace":         fmt.Sprintf("namespace-%d", rand.Intn(1000)),
		"k8s-app":                             fmt.Sprintf("app-%d", rand.Intn(1000)),
		"kubernetes.azure.com/managedby":      "aks",
	}
}

// generateRandomSecurityLabels creates random security labels for the CiliumIdentity.
func generateRandomSecurityLabels() map[string]string {
	return map[string]string{
		"k8s:io.cilium.k8s.namespace.labels.addonmanager.kubernetes.io/mode": "Reconcile",
		"k8s:io.cilium.k8s.namespace.labels.control-plane":                   "true",
		"k8s:io.cilium.k8s.namespace.labels.kubernetes.azure.com/managedby":  "aks",
		"k8s:io.cilium.k8s.namespace.labels.kubernetes.io/cluster-service":   "true",
		"k8s:io.cilium.k8s.namespace.labels.kubernetes.io/metadata.name":     fmt.Sprintf("namespace-%d", rand.Intn(1000)),
		"k8s:io.cilium.k8s.policy.cluster":                                   "default",
		"k8s:io.cilium.k8s.policy.serviceaccount":                            fmt.Sprintf("serviceaccount-%d", rand.Intn(1000)),
		"k8s:io.kubernetes.pod.namespace":                                    fmt.Sprintf("namespace-%d", rand.Intn(1000)),
		"k8s:k8s-app":                                                        fmt.Sprintf("app-%d", rand.Intn(1000)),
		"k8s:kubernetes.azure.com/managedby":                                 "aks",
	}
}
