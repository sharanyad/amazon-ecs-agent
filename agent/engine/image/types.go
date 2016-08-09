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
	"sync"
	"time"

	"github.com/aws/amazon-ecs-agent/agent/api"
	"github.com/cihub/seelog"
)

type Image struct {
	ImageID string
	Names   []string
	Size    int64
}

// ImageState represents a docker image
// and its state information such as containers associated with it
type ImageState struct {
	Image      *Image
	Containers []*api.Container `json:"-"`
	PulledAt   time.Time
	LastUsedAt time.Time
	UpdateLock sync.RWMutex
}

func (imageState *ImageState) UpdateContainerReference(container *api.Container) {
	imageState.UpdateLock.Lock()
	defer imageState.UpdateLock.Unlock()
	seelog.Infof("Updating container reference %v in Image State - %v", container.Name, imageState.Image.ImageID)
	imageState.Containers = append(imageState.Containers, container)
	imageState.LastUsedAt = time.Now()
}

func (imageState *ImageState) AddImageName(imageName string) {
	imageState.UpdateLock.Lock()
	defer imageState.UpdateLock.Unlock()
	if !imageState.HasImageName(imageName) {
		seelog.Infof("Adding image name- %v to Image state- %v", imageName, imageState.Image.ImageID)
		imageState.Image.Names = append(imageState.Image.Names, imageName)
	}
}

func (imageState *ImageState) HasNoAssociatedContainers() bool {
	return len(imageState.Containers) == 0
}

func (imageState *ImageState) UpdateImageState(container *api.Container) {
	imageState.AddImageName(container.Image)
	imageState.UpdateContainerReference(container)
}

func (imageState *ImageState) RemoveImageName(containerImageName string) {
	imageState.UpdateLock.Lock()
	defer imageState.UpdateLock.Unlock()
	for i, imageName := range imageState.Image.Names {
		if imageName == containerImageName {
			imageState.Image.Names = append(imageState.Image.Names[:i], imageState.Image.Names[i+1:]...)
		}
	}
}

func (imageState *ImageState) HasImageName(containerImageName string) bool {
	for _, imageName := range imageState.Image.Names {
		if imageName == containerImageName {
			return true
		}
	}
	return false
}
