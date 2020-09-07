# AWS Secret Injector

Forked from https://github.com/aws-samples/aws-secret-sidecar-injector - thankyou to the authors for the starting point their proof-of-concept provided.

_aws-secret-injector_ allows your containerized applications to consume secrets from AWS Secrets Manager. The solution makes use of a Kubernetes dynamic admission controller that injects an _init_ container upon creation/update of your pod. The init container expects that [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) is configured to retrieve the secret from AWS Secrets Manager. The admission controller creates an in-memory Kubernetes volume associated with the pod to store the secret.

## Prerequsites 
- An AWS account
- IRSA configured on your Kubernetes cluster

## Installation

### Deploying the admission controller

You can use the [helm charts](https://github.com/ecrousseau/aws-secret-injector/tree/master/manifests/helm) supplied in this repo to install the admission controller. Warning: these charts are not well tested. 

## Injecting secrets into your pods

Add the following annotations to your podSpec to inject secrets into your pod:

  ```secrets.aws.k8s/injectorWebhook: init-container```

  ```secrets.aws.k8s/secretArns: <comma-separated list of ARNs>```
  
The decrypted secrets are written to a volume named `secret-vol` mounted at `/injected-secrets` for all containers in the pod, with filenames matching the secret name. 

This repository contains an example Kubernetes deployment [manifest](https://github.com/ecrousseau/aws-secret-injector/blob/master/manifests/examples/webserver.yaml) to illustrate this usage.

## License

This software is licensed under the MIT-0 License. See the LICENSE file. This repository is maintained completely independently of Amazon or AWS.
