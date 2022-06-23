# Copyright 2017 Heptio Inc.
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

TARGET = eventrouter
GOTARGET = github.com/openshift/$(TARGET)
LOCAL_IMAGE_TAG=zwindler/logging-eventrouter:0.4.0
IMAGE_REPOSITORY_NAME=zwindler/logging-eventrouter:0.4.0

ifneq ($(VERBOSE),)
VERBOSE_FLAG = -v
endif
TESTARGS ?= $(VERBOSE_FLAG) -timeout 60s
TEST_PKGS ?= $(GOTARGET)/sinks/...
TEST = go test $(TEST_PKGS) $(TESTARGS)

build: fmt
	go build -mod=mod -o $(TARGET)
.PHONY: build

fmt:
	@echo gofmt

image:
	docker build  -f Dockerfile -t $(LOCAL_IMAGE_TAG) .
	docker tag $(LOCAL_IMAGE_TAG) $(IMAGE_REPOSITORY_NAME)
.PHONY: image

test:
	go test -mod=mod $(TEST_PKGS) $(TESTARGS)
.PHONY: test
