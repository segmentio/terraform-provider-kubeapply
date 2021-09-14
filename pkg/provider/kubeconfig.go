package provider

import (
	_ "embed"
	"os"
	"path/filepath"
	"text/template"

	log "github.com/sirupsen/logrus"
)

var (
	//go:embed templates/kubeconfig.yaml
	kubeConfigTemplateStr string

	kubeConfigTemplate *template.Template
)

func init() {
	kubeConfigTemplate = template.Must(
		template.New("kubeconfig").Parse(kubeConfigTemplateStr),
	)
}

// Exec contains the specification for executing a command to get a Kubernetes auth token.
type Exec struct {
	ApiVersion string
	Command    string
	Env        map[string]string
	Args       []string
}

func createKubeConfig(data resourceGetter, tempDir string) (string, error) {
	existingKubeConfigPath := data.Get("config_path").(string)
	if existingKubeConfigPath != "" {
		log.Info("Using existing kubeconfig")
		return existingKubeConfigPath, nil
	}

	kubeConfigPath := filepath.Join(tempDir, "kubeconfig.yaml")

	out, err := os.Create(kubeConfigPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	var exec *Exec

	execRows := data.Get("exec").([]interface{})
	if len(execRows) > 0 {
		execRow := execRows[0].(map[string]interface{})

		env := map[string]string{}
		for key, value := range execRow["env"].(map[string]interface{}) {
			env[key] = value.(string)
		}

		args := []string{}
		for _, arg := range execRow["args"].([]interface{}) {
			args = append(args, arg.(string))
		}

		exec = &Exec{
			ApiVersion: execRow["api_version"].(string),
			Command:    execRow["command"].(string),
			Env:        env,
			Args:       args,
		}
	}

	err = kubeConfigTemplate.Execute(
		out,
		struct {
			CAData         string
			ClientCertData string
			ClientKeyData  string
			Exec           *Exec
			Insecure       bool
			Password       string
			Server         string
			Token          string
			Username       string
		}{
			CAData:         data.Get("cluster_ca_certificate").(string),
			ClientCertData: data.Get("client_certificate").(string),
			ClientKeyData:  data.Get("client_key").(string),
			Exec:           exec,
			Insecure:       data.Get("insecure").(bool),
			Password:       data.Get("password").(string),
			Server:         data.Get("host").(string),
			Token:          data.Get("token").(string),
			Username:       data.Get("username").(string),
		},
	)
	if err != nil {
		return "", err
	}

	return kubeConfigPath, nil
}
