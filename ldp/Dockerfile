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

FROM golang:latest AS build

ADD . /go/src/github.com/membrane/local-directory-provisioner/ldp
WORKDIR /go/src/github.com/membrane/local-directory-provisioner/ldp
ENV CGO_ENABLED=0 GOOS=linux GOARCH=386
RUN go build -a -ldflags '-extldflags "-static"' -o /go/bin/local-directory-provisioner local-directory-provisioner.go


FROM alpine:latest

COPY --from=build /go/bin/local-directory-provisioner /usr/local/bin/local-directory-provisioner
CMD ["/usr/local/bin/local-directory-provisioner"]
