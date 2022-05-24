package kube

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type apiResource struct {
	name       string
	shortNames []string
	apiVersion string
	namespaced bool
	kind       string
}

var apiResourceLoader = loadApiResourcesFromCluster

func loadApiResourcesFromCluster(kubeConfigPath string) ([]*v1.APIResourceList, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}
	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	_, resourceLists, err := k8sClient.ServerGroupsAndResources()
	return resourceLists, err
}

func getApiResources(kubeConfigPath string) ([]apiResource, error) {
	resourceLists, err := apiResourceLoader(kubeConfigPath)
	if err != nil {
		return nil, err
	}
	outputResources := []apiResource{}
	for _, l := range resourceLists {
		for _, r := range l.APIResources {
			if r.Name != "" && l.APIVersion != "" && r.Kind != "" {
				outputResources = append(outputResources, apiResource{
					name:       r.Name,
					shortNames: r.ShortNames,
					apiVersion: l.APIVersion,
					namespaced: r.Namespaced,
					kind:       r.Kind,
				})
			}

		}
	}
	return outputResources, nil
}
