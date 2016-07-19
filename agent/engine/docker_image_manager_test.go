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

package engine

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-agent/agent/api"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/mock/gomock"
)

func TestAddAndRemoveContainerToImageStateReferenceHappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &Image{
		ImageId: "sha256:qwerty",
	}
	sourceImage.Names = append(sourceImage.Names, container.Image)
	sourceImageState := &ImageState{
		Image:      sourceImage,
		PulledTime: time.Now(),
	}
	imageManager.AddImageState(sourceImageState)
	imageinspect := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageinspect, nil).AnyTimes()
	err := imageManager.AddContainerReferenceToImageState(container)
	if err != nil {
		t.Error("Error in adding container to an existing image state")
	}
	client.EXPECT().InspectImage(container.Image).Return(imageinspect, nil).AnyTimes()
	imageState, ok := imageManager.(*dockerImageManager).getImageState(container)
	if !ok {
		t.Error("Error in retrieving existing Image State for the Container")
	}
	if !reflect.DeepEqual(sourceImageState, imageState) {
		t.Error("Mismatch between added and retrieved image state")
	}
	client.EXPECT().InspectImage(container.Image).Return(imageinspect, nil).AnyTimes()
	err = imageManager.RemoveContainerReferenceFromImageState(container)
	if err != nil {
		t.Error("Error removing container reference from image state")
	}
	client.EXPECT().InspectImage(container.Image).Return(imageinspect, nil).AnyTimes()
	imageState, _ = imageManager.(*dockerImageManager).getImageState(container)
	if len(imageState.Containers) != 0 {
		t.Error("Error removing container reference from image state")
	}
}

func TestAddContainerReferenceToInvalidImageState(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &Image{
		ImageId: "sha256:qwerty",
	}
	sourceImage.Names = append(sourceImage.Names, container.Image)
	sourceImageState := &ImageState{
		Image:      sourceImage,
		PulledTime: time.Now(),
	}
	imageManager.AddImageState(sourceImageState)
	client.EXPECT().InspectImage(container.Image).Return(nil, errors.New("error inspecting")).AnyTimes()
	client.EXPECT().InspectImage(container.Image).Return(nil, errors.New("error inspecting")).AnyTimes()
	err1 := imageManager.AddContainerReferenceToImageState(container)
	if err1 == nil {
		t.Error("Expected error in adding container to invalid image")
	}
}

func TestAddContainerReferenceToImageStateWithNoImageName(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &Image{
		ImageId: "sha256:qwerty",
	}
	sourceImageState := &ImageState{
		Image:      sourceImage,
		PulledTime: time.Now(),
	}
	imageManager.AddImageState(sourceImageState)
	imageinspect := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageinspect, nil).AnyTimes()
	err := imageManager.AddContainerReferenceToImageState(container)
	if err != nil {
		t.Error("Error in adding container to an existing image state")
	}
	client.EXPECT().InspectImage(container.Image).Return(imageinspect, nil).AnyTimes()
	imageState, ok := imageManager.(*dockerImageManager).getImageState(container)
	if !ok {
		t.Error("Error in retrieving existing Image State for the Container")
	}
	for _, imageName := range imageState.Image.Names {
		if imageName != container.Image {
			t.Error("Error while adding image name and container to image state")
		}
	}
}

func TestAddInvalidContainerReferenceToImageState(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Image: "",
	}
	err := imageManager.AddContainerReferenceToImageState(container)
	if err == nil {
		t.Error("Expected error adding invalid container reference to image state")
	}
}

func TestRemoveInvalidContainerReferenceFromImageState(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Image: "",
	}
	err := imageManager.RemoveContainerReferenceFromImageState(container)
	if err == nil {
		t.Error("Expected error removing invalid container reference from image state")
	}
}

func TestRemoveContainerReferenceFromInvalidImageState(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Image: "myContainerImage",
	}
	client.EXPECT().InspectImage(container.Image).Return(nil, errors.New("error inspecting")).AnyTimes()
	err := imageManager.RemoveContainerReferenceFromImageState(container)
	if err == nil {
		t.Error("Expected error removing container reference from invalid image state")
	}
}

func TestRemoveContainerReferenceFromImageStateWithNoReference(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &Image{
		ImageId: "sha256:qwerty",
	}
	sourceImageState := &ImageState{
		Image:      sourceImage,
		PulledTime: time.Now(),
	}
	imageManager.AddImageState(sourceImageState)
	imageinspect := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageinspect, nil).AnyTimes()
	err := imageManager.RemoveContainerReferenceFromImageState(container)
	if err == nil {
		t.Error("Expected error removing non-existing container reference from image state")
	}
}

func TestAddImageState(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	sourceImage := &Image{}
	sourceImage.Names = append(sourceImage.Names, "myImage")
	sourceImageState := &ImageState{
		Image:      sourceImage,
		PulledTime: time.Now(),
	}
	imageManager.AddImageState(sourceImageState)
	for _, imageState := range imageManager.(*dockerImageManager).getAllImageStates() {
		if !reflect.DeepEqual(sourceImageState, imageState) {
			t.Error("Error adding image state to image manager")
		}
	}
}
