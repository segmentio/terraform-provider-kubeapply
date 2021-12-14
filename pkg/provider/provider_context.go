package provider

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/diff"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/cluster/kube"
	"github.com/segmentio/terraform-provider-kubeapply/pkg/util"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// Default value used for sets where inputs are unknown
	defaultPlaceholder = `"DEFAULT_PLACEHOLDER_VALUE"`
)

type providerContext struct {
	allowDeletes         bool
	autoCreateNamespaces bool
	canRun               bool
	clusterClient        cluster.Client
	clusterConfig        cluster.Config
	createdAt            time.Time
	forceDiffs           bool
	keepExpanded         bool
	pid                  int
	rawClient            kubernetes.Interface
	showExpanded         bool
	sourceFetcher        *sourceFetcher
	tempDir              string
	verboseApplies       bool
	verboseDiffs         bool
}

type expandResult struct {
	expandedDir   string
	expandedFiles map[string]interface{}
	manifests     []kube.Manifest
	resources     map[string]interface{}
	totalHash     string
}

func (p *providerContext) expand(
	ctx context.Context,
	data resourceGetter,
) (*expandResult, error) {
	source := data.Get("source").(string)
	strParams := data.Get("parameters").(map[string]interface{})
	setParams := data.Get("set").(*schema.Set).List()

	clusterConfig := p.clusterConfig
	clusterConfig.Parameters = map[string]interface{}{}

	for key, value := range strParams {
		clusterConfig.Parameters[key] = value
	}

	for _, setParam := range setParams {
		rawMap := setParam.(map[string]interface{})
		name := rawMap["name"].(string)
		strValue := rawMap["value"].(string)

		if strValue == unknownValue {
			placeholder := rawMap["placeholder"].(string)

			if placeholder == "" {
				placeholder = defaultPlaceholder
			}

			log.Infof(
				"Using placeholder for key %s since value is unknown: %s",
				name,
				placeholder,
			)
			strValue = placeholder
		}

		var value interface{}
		if err := json.Unmarshal([]byte(strValue), &value); err != nil {
			return nil, err
		}
		clusterConfig.Parameters[name] = value
	}

	timeStamp := time.Now().UnixNano()
	expandedDir := filepath.Join(
		p.tempDir,
		"expanded",
		fmt.Sprintf("%d", timeStamp),
	)

	err := p.sourceFetcher.get(ctx, source, expandedDir)
	if err != nil {
		return nil, err
	}

	clusterConfig.ConfigHash, err = p.clusterConfigHash(
		expandedDir,
		clusterConfig.Parameters,
	)
	if err != nil {
		return nil, err
	}

	if err := util.ApplyTemplate(expandedDir, clusterConfig, true, true); err != nil {
		return nil, err
	}

	expandedFiles := map[string]interface{}{}

	err = filepath.Walk(expandedDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(expandedDir, path)
		if err != nil {
			return err
		}

		contents, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		expandedFiles[relPath] = string(contents)

		return nil
	})

	if err != nil {
		return nil, err
	}

	manifests, err := kube.GetManifests([]string{expandedDir})
	if err != nil {
		return nil, err
	}

	resources := map[string]interface{}{}
	for _, manifest := range manifests {
		resources[manifest.ID] = manifest.Hash
	}

	return &expandResult{
		expandedDir:   expandedDir,
		expandedFiles: expandedFiles,
		manifests:     manifests,
		resources:     resources,
		totalHash:     p.manifestsHash(manifests),
	}, nil
}

func (p *providerContext) shouldDiff(data resourceChanger) bool {
	if data.Get("no_diff").(bool) {
		// Diffs are explicitly turned off in the resource
		return false
	} else if p.forceDiffs {
		// Diffs are turned on at the provider
		return true
	} else {
		// There are resource changes
		return data.HasChange("resources")
	}
}

func (p *providerContext) diff(
	ctx context.Context,
	path string,
) ([]diff.Result, error) {
	return p.clusterClient.DiffStructured(ctx, []string{path}, false)
}

func (p *providerContext) apply(
	ctx context.Context,
	path string,
	moduleName string,
) diag.Diagnostics {
	var diags diag.Diagnostics
	results, err := p.clusterClient.Apply(ctx, []string{path}, false)

	log.Infof(
		"Apply results for %s (err=%+v): %s",
		moduleName,
		err,
		string(results),
	)

	if err != nil {
		diags = append(
			diags,
			diag.Diagnostic{
				Severity: diag.Error,
				Summary:  err.Error(),
				Detail:   string(results),
			},
		)
		return diags
	}

	if p.verboseApplies {
		// Add kubectl results as a "WARNING" so they show up in apply output
		diags = append(
			diags,
			diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  "kubectl apply successful",
				Detail:   prettyResults(results),
			},
		)
	}

	return diags
}

func (p *providerContext) canDelete(data resourceChanger) bool {
	return p.allowDeletes
}

func (p *providerContext) delete(
	ctx context.Context,
	data resourceChanger,
	ids []string,
) diag.Diagnostics {
	var diags diag.Diagnostics

	if !p.canDelete(data) {
		diags = append(
			diags,
			diag.Diagnostic{
				Severity: diag.Warning,
				Summary: fmt.Sprintf(
					"The kubeapply provider is not configured for deletes in %s; please delete manually if needed.",
					moduleName(data),
				),
			},
		)
		return diags
	}

	results, err := p.clusterClient.Delete(ctx, ids)
	log.Infof(
		"Delete results for %s (err=%+v): %s",
		moduleName(data),
		err,
		string(results),
	)

	if err != nil {
		diags = append(
			diags,
			diag.Diagnostic{
				Severity: diag.Error,
				Summary:  err.Error(),
				Detail:   string(results),
			},
		)
		return diags
	}

	diags = append(
		diags,
		diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  "kubectl delete successful",
			Detail:   string(results),
		},
	)
	return diags
}

func (p *providerContext) shouldShowExpanded(data resourceChanger) bool {
	return data.Get("show_expanded").(bool)
}

func (p *providerContext) createNamespaces(
	ctx context.Context,
	manifests []kube.Manifest,
) error {
	if !p.autoCreateNamespaces {
		log.Info("Not auto-creating namespaces since auto_create_namespaces is false")
		return nil
	}

	manifestNamespacesMap := map[string]struct{}{}

	for _, manifest := range manifests {
		if manifest.Head.Metadata.Namespace != "" {
			manifestNamespacesMap[manifest.Head.Metadata.Namespace] = struct{}{}
		}
	}

	apiNamespaces, err := p.rawClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	apiNamespacesMap := map[string]struct{}{}
	for _, namespace := range apiNamespaces.Items {
		apiNamespacesMap[namespace.Name] = struct{}{}
	}

	for namespace := range manifestNamespacesMap {
		if _, ok := apiNamespacesMap[namespace]; !ok {
			log.Infof("Namespace %s is in manifest but not API, creating", namespace)
			_, err = p.rawClient.CoreV1().Namespaces().Create(
				ctx,
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
					},
				},
				metav1.CreateOptions{},
			)
			if err != nil {
				if strings.Contains(err.Error(), "already exists") {
					// Swallow error
					log.Infof("Namespace %s already exists", namespace)
				} else {
					return err
				}
			}
		}
	}

	return nil
}

func (p *providerContext) manifestsHash(
	manifests []kube.Manifest,
) string {
	hash := md5.New()

	hash.Write([]byte(p.clusterConfig.Cluster))
	hash.Write([]byte(p.clusterConfig.Region))
	hash.Write([]byte(p.clusterConfig.AccountName))

	for _, manifest := range manifests {
		hash.Write([]byte(manifest.Hash))
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}

func (p *providerContext) cleanExpanded(
	result *expandResult,
) error {
	if p.keepExpanded {
		return nil
	}
	return os.RemoveAll(result.expandedDir)
}

func (p *providerContext) clusterConfigHash(
	sourcesPath string,
	parameters map[string]interface{},
) (string, error) {
	hash := md5.New()

	err := filepath.Walk(
		sourcesPath,
		func(subPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			contents, err := ioutil.ReadFile(subPath)
			if err != nil {
				return err
			}
			hash.Write(contents)
			return nil
		},
	)
	if err != nil {
		return "", err
	}

	parametersJson, err := json.Marshal(parameters)
	if err != nil {
		return "", err
	}

	hash.Write(parametersJson)
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
