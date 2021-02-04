# AWS Secret Injector

[![Build Status](https://github.com/ecrousseau/aws-secret-injector/workflows/master/badge.svg)](https://github.com/ecrousseau/aws-secret-injector/actions)

Forked from https://github.com/aws-samples/aws-secret-sidecar-injector - thankyou to the authors for the starting point their proof-of-concept provided.

_aws-secret-injector_ allows your containerized applications to consume secrets from AWS Secrets Manager. The solution makes use of a Kubernetes dynamic admission controller that injects an _init_ container upon creation/update of your pod. The init container expects that [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) is configured to retrieve the secret from AWS Secrets Manager. The admission controller creates an in-memory Kubernetes volume associated with the pod to store the secret.

## Prerequsites 
- An AWS account
- IRSA configured on your Kubernetes cluster

## Installation

### Deploying the admission controller

Grab the [latest release](https://github.com/ecrousseau/aws-secret-injector/releases) and use [Helm](https://helm.sh/) to install it.

```bash
$ kubectl create ns injector
$ helm install --namespace injector aws-secret-injector https://github.com/ecrousseau/aws-secret-injector/releases/download/v1.6/aws-secret-injector-v1.6.tgz
NAME: aws-secret-injector
LAST DEPLOYED: Fri Feb  5 01:09:20 2021
NAMESPACE: injector
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

Alternatively, create your own manifests using the [helm chart](https://github.com/ecrousseau/aws-secret-injector/tree/master/charts/aws-secret-injector) as a guide. The container images are published in GHCR [here](https://github.com/ecrousseau?tab=packages&q=aws-secret-injector).

## Usage

### Injecting secrets into your pods

Add the injectorWebhook annotation to your podSpec to inject secrets into your pod:

  ```secrets.aws.k8s/injectorWebhook: init-container```

And add either the secret ARNs:

  ```secrets.aws.k8s/secretArns: <comma-separated list of ARNs>```

Or the secret names and AWS region:

  ```secrets.aws.k8s/secretNames: <comma-separated list of friendly names>```

  ```secrets.aws.k8s/region: <AWS region for the secrets>```

(Optional) Set a flag to explode JSON into multiple files:

  ```secrets.aws.k8s/explodeJsonKeys: <true/false> ```

With this option set, the init container will interpret the secret value as JSON, and write one file for each key, with the content being the associated value. _Note: if you use this option, all secrets must be a string containing valid JSON._

### Notes 

If your secrets are spread across multiple regions you must use the ARN format. Note that the ARN does not need to include the "hash" - see the documentation on incomplete ARNs [here](https://docs.aws.amazon.com/sdk-for-go/api/service/secretsmanager/#GetSecretValueInput).
  
The decrypted secrets are written to a volume named `secret-vol` mounted at `/injected-secrets` for all containers in the pod, with filenames matching the secret name. 

This repository contains an example Kubernetes deployment [manifest](https://github.com/ecrousseau/aws-secret-injector/blob/master/examples/webserver.yaml) to demonstrate how to use the system.

### Additional configuration options settings

#### Proxy settings

You can configure the init container to use a proxy by creating a ConfigMap named "proxy-settings" in the namespace of your application (not the namespace of the admission controller), that contains keys "HTTPS_PROXY" and "NO_PROXY". These will be applied as environment variables in the init container.

#### Pre-existing volume

You can add a volume named "secret-vol" to your Pod spec. The init container will then write to that volume instead of the default in-memory volume. You may wish to do this if you need to mount the volume at a location other than `/injected-secrets`. Please ensure that the storage backing the volume you specify is secured appropriately!

## License

This software is licensed under the MIT-0 License. See the LICENSE file. This repository is maintained completely independently of Amazon or AWS.
