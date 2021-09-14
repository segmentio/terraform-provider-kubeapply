module github.com/segmentio/terraform-provider-kubeapply

go 1.16

require (
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/aws/aws-sdk-go v1.38.61 // indirect
	github.com/fatih/color v1.12.0
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/terraform-plugin-sdk/v2 v2.7.1
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/olekukonko/tablewriter v0.0.5
	github.com/pmezard/go-difflib v1.0.0
	github.com/segmentio/cli v0.4.0
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/x-cray/logrus-prefixed-formatter v0.5.2
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/klog/v2 v2.8.0
)
