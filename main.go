package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"time"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumclientset "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

	config.QPS = 100
	config.Burst = 200

	// Create Cilium clientset
	clientset, err := ciliumclientset.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Create Kubernetes core clientset
	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	wg := sync.WaitGroup{}
	for i := 0; i < *count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

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
			if err != nil && !apierrors.IsAlreadyExists(err) {
				panic(err.Error())
			}

			fmt.Printf("Created CiliumIdentity with ID: %d\n", identityID)

			// Generate random namespace and pod name
			namespace := fmt.Sprintf("namespace-%d", rand.Intn(100))
			podName := fmt.Sprintf("pod-%d", rand.Intn(10000))

			_, err = kubeClientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}, metav1.CreateOptions{})
			if err != nil && !apierrors.IsAlreadyExists(err) {
				panic(fmt.Errorf("failed to create namespace '%s': %v", namespace, err))
			}

			// Create a new CiliumEndpoint object
			ciliumEndpoint := &ciliumv2.CiliumEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
					Labels:    map[string]string{"paultest": "true"},
				},
				Status: ciliumv2.EndpointStatus{
					Identity: &ciliumv2.EndpointIdentity{
						ID:     int64(identityID),
						Labels: flattenlabels(securityLabels),
					},
					Networking: &ciliumv2.EndpointNetworking{
						Addressing: []*ciliumv2.AddressPair{
							{
								IPV4: generateRandomIPv4(),
								IPV6: generateRandomIPv6(),
							},
						},
					},
				},
			}

			// Create the CiliumEndpoint in Kubernetes
			_, err = clientset.CiliumV2().CiliumEndpoints(namespace).Create(ctx, ciliumEndpoint, metav1.CreateOptions{})
			if err != nil && !apierrors.IsAlreadyExists(err) {
				panic(err.Error())
			}

			fmt.Printf("Created CiliumEndpoint for Pod '%s' in Namespace '%s'\n", podName, namespace)

		}()
	}
	wg.Wait()
}

func flattenlabels(input map[string]string) []string {
	var labels []string
	for k, v := range input {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	return labels
}

// generateRandomLabels creates random labels for the CiliumIdentity metadata.
func generateRandomLabels() map[string]string {
	return map[string]string{
		"paultest":                            "true",
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

// generateRandomIPv4 generates a random IPv4 address.
func generateRandomIPv4() string {
	return fmt.Sprintf("10.%d.%d.%d", rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

// generateRandomIPv6 generates a random IPv6 address.
func generateRandomIPv6() string {
	return fmt.Sprintf("fd00:%x:%x::%x", rand.Intn(65536), rand.Intn(65536), rand.Intn(65536))
}
