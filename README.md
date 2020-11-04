# AWS Secret Injector

[![Build Status](https://github.com/ecrousseau/aws-secret-injector/workflows/master/badge.svg)](https://github.com/ecrousseau/aws-secret-injector/actions)

Forked from https://github.com/aws-samples/aws-secret-sidecar-injector - thankyou to the authors for the starting point their proof-of-concept provided.

_aws-secret-injector_ allows your containerized applications to consume secrets from AWS Secrets Manager. The solution makes use of a Kubernetes dynamic admission controller that injects an _init_ container upon creation/update of your pod. The init container expects that [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) is configured to retrieve the secret from AWS Secrets Manager. The admission controller creates an in-memory Kubernetes volume associated with the pod to store the secret.

## Prerequsites 
- An AWS account
- IRSA configured on your Kubernetes cluster

## Installation

### Deploying the admission controller

You can use the [helm charts](https://github.com/ecrousseau/aws-secret-injector/tree/master/manifests/helm) supplied in this repo to install the admission controller. Alternatively, you can use the templates within the helm chart as a guide to creating your own manifests.

## Usage

### Injecting secrets into your pods

Add the injectorWebhook annotation to your podSpec to inject secrets into your pod:

  ```secrets.aws.k8s/injectorWebhook: init-container```

And add either the secret ARNs:

  ```secrets.aws.k8s/secretArns: <comma-separated list of ARNs>```

Or the secret names and AWS region:

  ```secrets.aws.k8s/secretNames: <comma-separated list of friendly names>```

  ```secrets.aws.k8s/region: <AWS region for the secrets>```

Optional flag for exploding json format in secret manager into into individual file. This only works for exploding string type secrets (not binary) and that it applies to all secrets.

  ```secrets.aws.k8s/explodeJsonKeys: <true/false> ```

If your secrets are spread across multiple regions you must use the ARN format. Note that the ARN does not need to include the "hash" - see the documentation on incomplete ARNs [here](https://docs.aws.amazon.com/sdk-for-go/api/service/secretsmanager/#GetSecretValueInput).
  
The decrypted secrets are written to a volume named `secret-vol` mounted at `/injected-secrets` for all containers in the pod, with filenames matching the secret name. 

This repository contains an example Kubernetes deployment [manifest](https://github.com/ecrousseau/aws-secret-injector/blob/master/manifests/examples/webserver.yaml) to illustrate this usage.

### Optional settings

#### Proxy settings

You can configure the init container to use a proxy by creating a ConfigMap named "proxy-settings" that contains keys "HTTPS_PROXY" and "NO_PROXY". These will be applied as environment variables in the init container.

#### Pre-existing volume

You can add a volume named "secret-vol" to your Pod spec. The init container will then write to that volume instead of the default in-memory volume. You may wish to do this if you need to mount the volume at a location other than `/injected-secrets`. Please ensure that the storage backing the volume you specify is secured appropriately!

## License

This software is licensed under the MIT-0 License. See the LICENSE file. This repository is maintained completely independently of Amazon or AWS.
