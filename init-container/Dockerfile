# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang AS builder
WORKDIR /app
COPY . .
RUN go get -v
RUN CGO_ENABLED=0 go build -o injector

FROM ubuntu AS certs
RUN  apt-get update && apt-get install -y ca-certificates

FROM scratch
COPY --from=builder /app/injector /app/injector
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
ENTRYPOINT ["/app/injector"]