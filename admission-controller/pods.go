/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
    "fmt"
    admission "k8s.io/api/admission/v1"
    core "k8s.io/api/core/v1"
    meta "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    "k8s.io/klog/v2"
    "encoding/json"
)

var (
    False = false
    True = true
)

type Patch struct {
    Op string `json:"op"`
    Path string `json:"path"`
    Value interface{} `json:"value"`
}

func hasContainer(containers []core.Container, containerName string) bool {
    for _, container := range containers {
        if container.Name == containerName {
            return true
        }
    }
    return false
}

func hasVolume(volumes []core.Volume, volumeName string) bool {
    for _, volume := range volumes {
        if volume.Name == volumeName {
            return true
        }
    }
    return false
}

func getRoleArn(containers []core.Container) (string, error) {
    for _, container := range containers {
        for _, envVar := range container.Env {
            if envVar.Name == "AWS_ROLE_ARN" {
                return envVar.Value, nil
            }
        }
    }
    return "", fmt.Errorf("Unable to determine value for AWS_ROLE_ARN")
}

func mutatePods(ar admission.AdmissionReview) *admission.AdmissionResponse {
    klog.Info("Mutating pods")
    /* prepare the response */
    reviewResponse := admission.AdmissionResponse{
        Allowed: true,
        UID: ar.Request.UID,
    }

    /* examine the request */
    podResourceType := meta.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
    if ar.Request.Resource != podResourceType {
        klog.Error("Unexpected resource type ", ar.Request.Resource)
        return &reviewResponse  //something is wonky on the Kubernetes side - just send back an "Allow"
    }

    /* deserialize the raw request into a pod object */
    raw := ar.Request.Object.Raw
    pod := core.Pod{}
    deserializer := codecs.UniversalDeserializer()
    if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
        klog.Error("Unable to decode pod object: ", err)
        return toV1AdmissionResponse(err, ar)
    }

    /* examine the injectorWebhook annotation */
    klog.Info("Pod annotations:", pod.ObjectMeta.Annotations)
    annotation_injector_webhook, ok := pod.ObjectMeta.Annotations["secrets.aws.k8s/injectorWebhook"]
    if !ok {
        klog.Info("Pod annotation secrets.aws.k8s/injectorWebhook not set - no action required")
        return &reviewResponse
    }
    klog.Info("Pod annotation secrets.aws.k8s/injectorWebhook is set to ", annotation_injector_webhook)

    /* decide how to patch the pod */
    /* TODO add sidecar option */
    if annotation_injector_webhook == "init-container" {
        klog.Info("Injecting init container")
        if hasContainer(pod.Spec.InitContainers, "secrets-init-container") {
            err := "Pod already has an init container named secrets-init-container"
            klog.Error(err)
            return toV1AdmissionResponse(fmt.Errorf("%s", err), ar)
        }
        
        var patches []Patch

        /* add init container patch */
        env := []core.EnvVar{
            core.EnvVar{
                Name: "HTTPS_PROXY",
                ValueFrom: &core.EnvVarSource{
                    ConfigMapKeyRef: &core.ConfigMapKeySelector{
                        LocalObjectReference: core.LocalObjectReference{
                            Name: "proxy-settings",
                        },
                        Key: "HTTPS_PROXY",
                        Optional: &False,
                    },
                },
            },
            core.EnvVar{
                Name: "NO_PROXY",
                ValueFrom: &core.EnvVarSource{
                    ConfigMapKeyRef: &core.ConfigMapKeySelector{
                        LocalObjectReference: core.LocalObjectReference{
                            Name: "proxy-settings",
                        },
                        Key: "NO_PROXY",
                        Optional: &False,
                    },
                },
            },
            core.EnvVar{
                Name: "AWS_STS_REGIONAL_ENDPOINTS", 
                Value: "regional",
            },
        }
        _, secretArnsSet := pod.ObjectMeta.Annotations["secrets.aws.k8s/secretArns"]
        _, secretNamesSet := pod.ObjectMeta.Annotations["secrets.aws.k8s/secretNames"]
        _, explodeJsonKeysSet := pod.ObjectMeta.Annotations["secrets.aws.k8s/explodeJsonKeys"]
        annotation_region, regionSet := pod.ObjectMeta.Annotations["secrets.aws.k8s/region"]
        if secretArnsSet && secretNamesSet {
            err := "Only one of pod annotations secrets.aws.k8s/secretArns and secrets.aws.k8s/secretNames can be set"
            klog.Error(err)
            return toV1AdmissionResponse(fmt.Errorf("%s", err), ar)
        }
        if !secretArnsSet && !secretNamesSet {
            err := "One of pod annotations secrets.aws.k8s/secretArns or secrets.aws.k8s/secretNames must be set"
            klog.Error(err)
            return toV1AdmissionResponse(fmt.Errorf("%s", err), ar)
        }
        if secretArnsSet {
            if regionSet {
                klog.Warning("Pod annotation secrets.aws.k8s/secretArns is set, so secrets.aws.k8s/region will be ignored")
            }
            env = append(env, core.EnvVar{
                Name: "SECRET_ARNS",
                ValueFrom: &core.EnvVarSource{
                    FieldRef: &core.ObjectFieldSelector{
                        FieldPath: "metadata.annotations['secrets.aws.k8s/secretArns']",
                    },
                },
            })
        } else if secretNamesSet {
            if !regionSet {
                err := "Pod annotation secrets.aws.k8s/secretNames requires that annotation secrets.aws.k8s/region is also set"
                klog.Error(err)
                return toV1AdmissionResponse(fmt.Errorf("%s", err), ar)
            } else {
                env = append(env, core.EnvVar{
                    Name: "SECRET_REGION",
                    Value: annotation_region,
                })
                env = append(env, core.EnvVar{
                    Name: "SECRET_NAMES", 
                    ValueFrom: &core.EnvVarSource{
                        FieldRef: &core.ObjectFieldSelector{
                            FieldPath: "metadata.annotations['secrets.aws.k8s/secretNames']",
                        },
                    },
                })
            }
        }
        if explodeJsonKeysSet {
            env = append(env, core.EnvVar{
                Name: "EXPLODE_JSON_KEYS", 
                ValueFrom: &core.EnvVarSource{
                    FieldRef: &core.ObjectFieldSelector{
                        FieldPath: "metadata.annotations['secrets.aws.k8s/explodeJsonKeys']",
                    },
                },
            })
        }
        volumeMounts := []core.VolumeMount{
            core.VolumeMount{
                Name: "secret-vol",
                MountPath: "/injected-secrets",
                ReadOnly: false,
            },
        }
        if hasVolume(pod.Spec.Volumes, "aws-iam-token") {
            /* pod has already been through the IRSA webhook, so we need to do some work */
            volumeMounts = append(volumeMounts, core.VolumeMount{
                Name: "aws-iam-token",
                MountPath: "/var/run/secrets/eks.amazonaws.com/serviceaccount",
                ReadOnly: true,
            })
            roleArn, err := getRoleArn(pod.Spec.Containers)
            if err != nil {
                return toV1AdmissionResponse(fmt.Errorf("%s", err), ar)
            }
            env = append(env, core.EnvVar{
                Name: "AWS_ROLE_ARN",
                Value: roleArn,
            })
            env = append(env, core.EnvVar{
                Name: "AWS_WEB_IDENTITY_TOKEN_FILE",
                Value: "/var/run/secrets/eks.amazonaws.com/serviceaccount/token",
            })
        }
        /* create path /spec/initContainers if its missing */
        if len(pod.Spec.InitContainers) == 0 {
            initContainers := make([]core.Container, 0) /* using make ensures the resulting JSON is [] */
            patches = append(patches, Patch{
                Op: "add",
                Path: "/spec/initContainers",
                Value: initContainers,
            })
        }
        patches = append(patches, Patch{
            Op: "add",
            Path: "/spec/initContainers/0",
            Value: core.Container{
                Name: "secrets-init-container",
                Image: config.InitContainerImage,
                VolumeMounts: volumeMounts,
                Env: env,
                Resources: core.ResourceRequirements{
                    Requests: core.ResourceList{
                        "CPU": resource.MustParse("100m"),
                        "Memory": resource.MustParse("128Mi"),
                    },
                    Limits: core.ResourceList{
                        "CPU": resource.MustParse("100m"),
                        "Memory": resource.MustParse("256Mi"),
                    },
                },
                SecurityContext: &core.SecurityContext{
                    ReadOnlyRootFilesystem: &True,
                    AllowPrivilegeEscalation: &False,
                    Privileged: &False,
                },
            },
        })

        /* add patches for each container */
        for i := range pod.Spec.Containers {
            patches = append(patches, Patch{
                Op: "add",
                Path: fmt.Sprintf("/spec/containers/%d/volumeMounts/-", i),
                Value: core.VolumeMount{
                    Name: "secret-vol",
                    MountPath: "/injected-secrets",
                    ReadOnly: false,
                },
            })
        }
        
        /* add patch to add volume 'secret-vol' if required */
        if hasVolume(pod.Spec.Volumes, "secret-vol") {
            klog.Info("Pod already has a volume named secret-vol. Secrets will be written to that volume.")
        } else {
            klog.Info("Adding an in-memory volume named secret-vol. Secrets will be written to that volume.")
            patches = append(patches, Patch{
                Op: "add",
                Path: "/spec/volumes/-",
                Value: core.Volume{
                    Name: "secret-vol",
                    VolumeSource: core.VolumeSource{
                        EmptyDir: &core.EmptyDirVolumeSource{
                            Medium: "Memory",
                        },
                    },
                },
            })
        }
        
        /* reconstruct the JSON string */    
        patchBytes, err := json.Marshal(patches)
        if err != nil {
            klog.Error("Error marshalling JSON: ", err)
            return toV1AdmissionResponse(err, ar)
        }
        reviewResponse.Patch = patchBytes
        patchType := admission.PatchTypeJSONPatch
        reviewResponse.PatchType = &patchType
        klog.Info("Patch: ", string(patchBytes))
    }

    /* send the response */
    return &reviewResponse
}