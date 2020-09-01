# How to

## Admission Controller

Example request like

```json
{
  "apiVersion": "admission.k8s.io/v1",
  "kind": "AdmissionReview",
  "request": {
    "uid": "705ab4f5-6393-11e8-b7cc-42010a800002",
    "kind": {"group":"","version":"v1","kind":"Pod"},
    "resource": {"group":"","version":"v1","resource":"pods"},
    "requestKind": {"group":"","version":"v1","kind":"Pod"},
    "requestResource": {"group":"","version":"v1","resource":"pods"},
    "name": "my-pod",
    "namespace": "my-namespace",
    "operation": "CREATE",
    "userInfo": {
      "username": "admin",
      "uid": "014fbff9a07c",
      "groups": ["system:authenticated","my-admin-group"],
      "extra": {
        "some-key":["some-value1", "some-value2"]
      }
    },
    "object": {"apiVersion":"v1","kind":"Pod","metadata":{"name": "my-pod", "annotations":{"secrets.aws.k8s/injectorWebhook": "init-container"}},"spec":{"containers":[{"name": "my-container", "image": "my-image:latest"}]}},
    "oldObject": null,
    "options": {"apiVersion":"meta.k8s.io/v1","kind":"CreateOptions"},
    "dryRun": false
  }
}
```

Build container

```
docker build admission-controller -t admc
```

Run container (/path/to/tls should contain an SSL cert/key)

```
docker run -p 8082:443 -v /path/to/tls:/tls admc --tls-cert-file=/tls/cert.pem --tls-private-key-file=/tls/cert-key.pem --init-container-image my-init-image
```

Send example request using curl

```
curl -sS -k -H 'Content-Type: application/json' --data-raw "$(cat example-request.json)" https://127.0.0.1:8082/mutating-pods | jq -r '.response.patch' | base64 -d | jq
```

Result should be like

```json
[
  {
    "op": "add",
    "path": "/spec/initContainers",
    "value": [
      {
        "env": [
          {
            "name": "SECRET_ARNS",
            "valueFrom": {
              "fieldRef": {
                "fieldPath": "metadata.annotations['secrets.aws.k8s/secretArns']"
              }
            }
          }
        ],
        "image": "%v",
        "name": "secrets-init-container",
        "resources": {
          "limits": {
            "cpu": "1",
            "memory": "256Mi"
          },
          "requests": {
            "cpu": "1",
            "memory": "256Mi"
          }
        },
        "volumeMounts": [
          {
            "mountPath": "/injected-secrets",
            "name": "secret-vol"
          }
        ]
      }
    ]
  },
  {
    "op": "add",
    "path": "/spec/volumes/-",
    "value": {
      "emptyDir": {
        "medium": "Memory"
      },
      "name": "secret-vol"
    }
  },
  {
    "op": "add",
    "path": "/spec/containers/0/volumeMounts/-",
    "value": {
      "mountPath": "/injected-secrets",
      "name": "secret-vol"
    }
  },
  {
    "op": "add",
    "path": "/spec/containers/0/volumeMounts/-",
    "value": {
      "name": "secret-vol",
      "mountPath": "/injected-secrets"
    }
  }
]
```

## Init Container

Build

```
docker build init-container -t ic
```

Test

```
docker run -e SECRET_ARNS=arn:aws:secretsmanager:us-east-1:123456789012:secret:database-password-hlRvvF,arn:aws:secretsmanager:us-east-1:123456789012:secret:database-password-hlRvvG ic
```

Should be like

```
I0805 02:50:32.895704       1 main.go:20] SECRET_ARNS env var is arn:aws:secretsmanager:us-east-1:123456789012:secret:database-password-hlRvvF,arn:aws:secretsmanager:us-east-1:123456789012:secret:database-password-hlRvvG
I0805 02:50:32.895768       1 main.go:24] Processing:arn:aws:secretsmanager:us-east-1:123456789012:secret:database-password-hlRvvF
E0805 02:50:36.213572       1 main.go:58] NoCredentialProviders: no valid providers in chain. Deprecated.
	For verbose messaging see aws.Config.CredentialsChainVerboseErrors
I0805 02:50:36.213595       1 main.go:24] Processing:arn:aws:secretsmanager:us-east-1:123456789012:secret:database-password-hlRvvG
E0805 02:50:36.546553       1 main.go:58] NoCredentialProviders: no valid providers in chain. Deprecated.
	For verbose messaging see aws.Config.CredentialsChainVerboseErrors
```