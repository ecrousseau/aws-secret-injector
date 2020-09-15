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
    "k8s.io/api/admission/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/klog"
    "encoding/json"
)

const init_container_patch string = `[
    {
        "op":"add",
        "path":"/spec/initContainers",
        "value":[
            {
                "image":"%v",
                "name":"secrets-init-container",
                "volumeMounts":[
                    {
                        "name":"secret-vol",
                        "mountPath":"/injected-secrets"
                    }
                ],
                "env":[
                    {
                        "name": "SECRET_ARNS",
                        "valueFrom": {
                            "fieldRef": {
                                "fieldPath": "metadata.annotations['secrets.aws.k8s/secretArns']"
                            }
                        }
                    },
                    {
                        "name": "HTTPS_PROXY",
                        "valueFrom": {
                            "configMapKeyRef": {
                                "name": "proxy-settings",
                                "key": "HTTPS_PROXY",
                                "optional": true
                            }
                        }
                    },
                    {
                        "name": "NO_PROXY",
                        "valueFrom": {
                            "configMapKeyRef": {
                                "name": "proxy-settings",
                                "key": "NO_PROXY",
                                "optional": true
                            }
                        }
                    }
                ],
                "resources":{
                    "requests":{
                        "cpu": "100m",
                        "memory": "256Mi"
                    },
                    "limits":{
                        "cpu": "100m",
                        "memory": "256Mi"
                    }
                }
            }
        ]
    }
]`

type EmptyDir struct {
    Medium string `json:"medium"`
}

type Volume struct {
    Name string `json:"name"`
    EmptyDir interface{} `json:"emptyDir"`
}

type VolumeMount struct {
    Name string `json:"name"`
    MountPath string `json:"mountPath"`
}

type Patch struct {
    Op string `json:"op"`
    Path string `json:"path"`
    Value interface{} `json:"value"`
}

func hasContainer(containers []corev1.Container, containerName string) bool {
    for _, container := range containers {
        if container.Name == containerName {
            return true
        }
    }
    return false
}

func hasVolume(volumes []corev1.Volume, volumeName string) bool {
    for _, volume := range volumes {
        if volume.Name == volumeName {
            return true
        }
    }
    return false
}

func mutatePods(ar v1.AdmissionReview) *v1.AdmissionResponse {
    klog.V(2).Info("mutating pods")
    podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
    if ar.Request.Resource != podResource {
        klog.Errorf("expect resource to be %s", podResource)
        return nil
    }
    /* deserialize the raw request into a pod object */
    raw := ar.Request.Object.Raw
    pod := corev1.Pod{}
    deserializer := codecs.UniversalDeserializer()
    if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
        klog.Error(err)
        return toV1AdmissionResponse(err)
    } else {
        klog.Info("Pod spec:", pod)
    }

    /* prepare the response */
    reviewResponse := v1.AdmissionResponse{}
    reviewResponse.Allowed = true

    /* examine the injectorWebhook annotation */
    klog.Info("Pod annotations:", pod.ObjectMeta.Annotations)
    annotation_injector_webhook, ok := pod.ObjectMeta.Annotations["secrets.aws.k8s/injectorWebhook"]
    if !ok { 
        klog.Info("Pod annotation secrets.aws.k8s/injectorWebhook not set - no action required")
        return &reviewResponse
    }
    klog.Info("Pod annotation secrets.aws.k8s/injectorWebhook is set to %s", annotation_injector_webhook)

    /* decide how to patch the pod */
    /* TODO add sidecar option */
    if annotation_injector_webhook == "init-container" {
        klog.Info("Injecting init container")
        if hasContainer(pod.Spec.InitContainers, "secrets-init-container") {
            klog.Error("Pod already has an init-container named secrets-init-container")
            return nil
        }

        /* read the JSON string */
        var patches []Patch
        json.Unmarshal([]byte(fmt.Sprintf(init_container_patch, sidecarImage)), &patches)
    
        /* add patches for each container */
        vm := VolumeMount{"secret-vol", "/injected-secrets"}
        for i := range pod.Spec.Containers {
            patch := Patch{"add", fmt.Sprintf("/spec/containers/%d/volumeMounts/-", i), vm}
            patches = append(patches, patch)
        }
        
        /* add patch to add volume 'secret-vol' if required */
        if hasVolume(pod.Spec.Volumes, "secret-vol") {
            klog.Info("Pod already has a volume named secret-vol. Secrets will be written to that volume.")
        } else {
            klog.Info("Adding an in-memory volume named secret-vol. Secrets will be written to that volume.")
            ed := EmptyDir{"Memory"}
            v := Volume{"secret-vol", ed}
            patch := Patch{"add", "/spec/volumes/-", v}
            patches = append(patches, patch)
        }
        
        /* reconstruct the JSON string */    
        b, err := json.Marshal(patches)
        if err != nil {
            fmt.Printf(err.Error())
        }
        reviewResponse.Patch = b
        pt := v1.PatchTypeJSONPatch
        reviewResponse.PatchType = &pt
        klog.Info("Patch: ", string(b))
    }

    /* log and send the response */
    klog.Info(reviewResponse.String())
    return &reviewResponse
}