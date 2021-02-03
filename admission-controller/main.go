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
    "encoding/json"
    "flag"
    "fmt"
    "io/ioutil"
    "net/http"

    admission "k8s.io/api/admission/v1"
    "k8s.io/klog/v2"
)

var (
    config Config
)

// handle the http and decoding portion of a request
func serveMutatePods(w http.ResponseWriter, r *http.Request) {
    var body []byte
    if r.Body != nil {
        if data, err := ioutil.ReadAll(r.Body); err == nil {
            body = data
        }
    }

    // verify the content type is correct
    contentType := r.Header.Get("Content-Type")
    if contentType != "application/json" {
        klog.Errorf("contentType=%s, expect application/json", contentType)
        return
    }

    klog.V(2).Infof("Handling request: %s", body)

    // decode the request
    deserializer := codecs.UniversalDeserializer()
    obj, gvk, err := deserializer.Decode(body, nil, nil)
    if err != nil {
        msg := fmt.Sprintf("Request could not be decoded: %v", err)
        klog.Error(msg)
        http.Error(w, msg, http.StatusBadRequest)
        return
    }

    // generate an AdmissionReview response based on the request
    var response admission.AdmissionReview
    switch *gvk {
    case admission.SchemeGroupVersion.WithKind("AdmissionReview"):
        request, ok := obj.(*admission.AdmissionReview)
        if !ok {
            klog.Errorf("Expected admission.AdmissionReview but got: %T", obj)
            return
        }
        response.SetGroupVersionKind(*gvk)
        response.Response = mutatePods(*request)
        response.Response.UID = request.Request.UID
    default:
        msg := fmt.Sprintf("Unsupported group version kind: %v", gvk)
        klog.Error(msg)
        http.Error(w, msg, http.StatusBadRequest)
        return
    }

    klog.V(2).Infof("Sending response: %v", response)
    responseBytes, err := json.Marshal(response)
    if err != nil {
        klog.Error(err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    if _, err := w.Write(responseBytes); err != nil {
        klog.Error(err)
    }
}

func main() {
    klog.InitFlags(&flag.FlagSet{})
    flag.Parse()

    http.HandleFunc("/mutating-pods", serveMutatePods)
    http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
    klog.Fatal(http.ListenAndServeTLS(":8443", config.CertFile, config.KeyFile, nil))
}
