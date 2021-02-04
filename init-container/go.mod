module github.com/ecrousseau/aws-secret-injector/init-container

go 1.15

require (
	github.com/aws/aws-sdk-go-v2 v1.1.0
	github.com/aws/aws-sdk-go-v2/config v1.1.0
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.1.0
	k8s.io/klog/v2 v2.5.0
)
