// Copyright 2014-2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//	http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package image

import (
	"time"

	"github.com/aws/amazon-ecs-agent/agent/api"
)

type Image struct {
	ImageId string
	Names   []string
	Size    int64
}

// ImageState represents a docker image
// and its state information such as containers associated with it
type ImageState struct {
	Image      *Image
	Containers []*api.Container
	PulledAt   time.Time
	LastUsedAt time.Time
}
