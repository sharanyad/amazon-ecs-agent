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
	"golang.org/x/net/context"
)

func TestAddAndRemoveContainerToImageStateReferenceHappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
		Image:    sourceImage,
		PulledAt: time.Now().AddDate(0, -2, 0),
	}
	sourceImageState.addImageName(container.Image)
	imageManager.(*dockerImageManager).addImageState(sourceImageState)
	imageInspected := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil)
	err := imageManager.AddContainerReferenceToImageState(container)
	if err != nil {
		t.Error("Error in adding container to an existing image state")
	}
	imageState, ok := imageManager.(*dockerImageManager).getImageState(imageInspected.ID)
	if !ok {
		t.Error("Error in retrieving existing Image State for the Container")
	}
	if !reflect.DeepEqual(sourceImageState, imageState) {
		t.Error("Mismatch between added and retrieved image state")
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil)
	err = imageManager.RemoveContainerReferenceFromImageState(container)
	if err != nil {
		t.Error("Error removing container reference from image state")
	}
	imageState, _ = imageManager.(*dockerImageManager).getImageState(imageInspected.ID)
	if len(imageState.Containers) != 0 {
		t.Error("Error removing container reference from image state")
	}

	imageStateForDeletion := imageManager.(*dockerImageManager).getCandidateImagesForDeletion()
	if !reflect.DeepEqual(imageStateForDeletion[0], sourceImageState) {
		t.Error("Mismatch between added and retrieved image state for deletion")
	}
	leastRecentlyUsedImage := imageManager.(*dockerImageManager).getLeastRecentlyUsedImages(imageStateForDeletion)
	if !reflect.DeepEqual(imageStateForDeletion[0], leastRecentlyUsedImage[0]) {
		t.Error("Mismatch between added and retrieved LRU image state for deletion")
	}
}

func TestAddContainerReferenceToImageStateInspectError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
		Image:    sourceImage,
		PulledAt: time.Now(),
	}
	sourceImageState.addImageName(container.Image)
	imageManager.(*dockerImageManager).addImageState(sourceImageState)
	client.EXPECT().InspectImage(container.Image).Return(nil, errors.New("error inspecting")).AnyTimes()
	err := imageManager.AddContainerReferenceToImageState(container)
	if err == nil {
		t.Error("Expected error in inspecting image while adding container to image state")
	}
}

func TestAddContainerReferenceToImageStateWithNoImageName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
		Image:    sourceImage,
		PulledAt: time.Now(),
	}
	imageManager.(*dockerImageManager).addImageState(sourceImageState)
	imageInspected := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil).AnyTimes()
	err := imageManager.AddContainerReferenceToImageState(container)
	if err != nil {
		t.Error("Error in adding container to an existing image state")
	}
	imageState, ok := imageManager.(*dockerImageManager).getImageState(imageInspected.ID)
	if !ok {
		t.Error("Error in retrieving existing Image State for the Container")
	}
	for _, imageName := range imageState.Image.Names {
		if imageName != container.Image {
			t.Error("Error while adding image name to image state")
		}
	}
}

func TestAddInvalidContainerReferenceToImageState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Image: "",
	}
	err := imageManager.AddContainerReferenceToImageState(container)
	if err == nil {
		t.Error("Expected error adding container reference with no image name to image state")
	}
}

func TestRemoveContainerReferenceFromInvalidImageState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Image: "myContainerImage",
	}
	imageInspected := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil).AnyTimes()
	err := imageManager.RemoveContainerReferenceFromImageState(container)
	if err == nil {
		t.Error("Expected error while adding container to an invalid image state")
	}
}

func TestRemoveInvalidContainerReferenceFromImageState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Image: "",
	}
	err := imageManager.RemoveContainerReferenceFromImageState(container)
	if err == nil {
		t.Error("Expected error removing container reference with no image name from image state")
	}
}

func TestRemoveContainerReferenceFromImageStateInspectError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Image: "myContainerImage",
	}
	client.EXPECT().InspectImage(container.Image).Return(nil, errors.New("error inspecting")).AnyTimes()
	err := imageManager.RemoveContainerReferenceFromImageState(container)
	if err == nil {
		t.Error("Expected error in inspecting image while adding container to image state")
	}
}

func TestRemoveContainerReferenceFromImageStateWithNoReference(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
		Image:    sourceImage,
		PulledAt: time.Now(),
	}
	imageManager.(*dockerImageManager).addImageState(sourceImageState)
	imageInspected := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil).AnyTimes()
	err := imageManager.RemoveContainerReferenceFromImageState(container)
	if err == nil {
		t.Error("Expected error removing non-existing container reference from image state")
	}
}

func TestGetCandidateImagesForDeletionImageNoImageState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	imageStates := imageManager.(*dockerImageManager).getCandidateImagesForDeletion()
	if imageStates != nil {
		t.Error("Expected no image state to be returned for deletion")
	}
}

func TestGetCandidateImagesForDeletionImageJustPulled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	sourceImage := &Image{}
	sourceImageState := &ImageState{
		Image:    sourceImage,
		PulledAt: time.Now(),
	}
	imageManager.(*dockerImageManager).addImageState(sourceImageState)
	imageStates := imageManager.(*dockerImageManager).getCandidateImagesForDeletion()
	if len(imageStates) > 0 {
		t.Error("Expected no image state to be returned for deletion")
	}
}

func TestGetCandidateImagesForDeletionImageHasContainerReference(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
		Image:    sourceImage,
		PulledAt: time.Now().AddDate(0, -2, 0),
	}
	imageManager.(*dockerImageManager).addImageState(sourceImageState)
	imageInspected := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil).AnyTimes()
	err := imageManager.AddContainerReferenceToImageState(container)
	if err != nil {
		t.Error("Error in adding container to an existing image state")
	}
	imageStates := imageManager.(*dockerImageManager).getCandidateImagesForDeletion()
	if len(imageStates) > 0 {
		t.Error("Expected no image state to be returned for deletion")
	}
}

func TestGetCandidateImagesForDeletionImageHasMoreContainerReferences(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	container2 := &api.Container{
		Name:  "testContainer2",
		Image: "testContainerImage",
	}
	sourceImage := &Image{
		ImageId: "sha256:qwerty",
	}
	sourceImage.Names = append(sourceImage.Names, container.Image)
	sourceImageState := &ImageState{
		Image:    sourceImage,
		PulledAt: time.Now().AddDate(0, -2, 0),
	}
	imageManager.(*dockerImageManager).addImageState(sourceImageState)
	imageInspected := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil).AnyTimes()
	err := imageManager.AddContainerReferenceToImageState(container)
	if err != nil {
		t.Error("Error in adding container to an existing image state")
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil).AnyTimes()
	err = imageManager.AddContainerReferenceToImageState(container2)
	if err != nil {
		t.Error("Error in adding container2 to an existing image state")
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil).AnyTimes()
	err = imageManager.RemoveContainerReferenceFromImageState(container)
	if err != nil {
		t.Error("Error removing container reference from image state")
	}
	imageStates := imageManager.(*dockerImageManager).getCandidateImagesForDeletion()
	if len(imageStates) > 0 {
		t.Error("Expected no image state to be returned for deletion")
	}
}

func TestGetLeastRecentlyUsedImages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	imageStateA := &ImageState{
		LastUsedAt: time.Now().AddDate(0, -5, 0),
	}
	imageStateB := &ImageState{
		LastUsedAt: time.Now().AddDate(0, -3, 0),
	}
	imageStateC := &ImageState{
		LastUsedAt: time.Now().AddDate(0, -2, 0),
	}
	imageStateD := &ImageState{
		LastUsedAt: time.Now().AddDate(0, -6, 0),
	}
	imageStateE := &ImageState{
		LastUsedAt: time.Now().AddDate(0, -4, 0),
	}
	imageStateF := &ImageState{
		LastUsedAt: time.Now().AddDate(0, -1, 0),
	}

	candidateImagesForDeletion := []*ImageState{
		imageStateA, imageStateB, imageStateC, imageStateD, imageStateE, imageStateF,
	}
	expectedLeastRecentlyUsedImages := []*ImageState{
		imageStateD, imageStateA, imageStateE, imageStateB, imageStateC,
	}
	leastRecentlyUsedImages := imageManager.(*dockerImageManager).getLeastRecentlyUsedImages(candidateImagesForDeletion)
	for i := range leastRecentlyUsedImages {
		if !reflect.DeepEqual(leastRecentlyUsedImages[i], expectedLeastRecentlyUsedImages[i]) {
			t.Error("Incorrect order of least recently used images")
		}
	}
}

func TestGetLeastRecentlyUsedImagesLessThanFive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client)
	imageStateA := &ImageState{
		LastUsedAt: time.Now().AddDate(0, -5, 0),
	}
	imageStateB := &ImageState{
		LastUsedAt: time.Now().AddDate(0, -3, 0),
	}
	imageStateC := &ImageState{
		LastUsedAt: time.Now().AddDate(0, -2, 0),
	}
	candidateImagesForDeletion := []*ImageState{
		imageStateA, imageStateB, imageStateC,
	}
	expectedLeastRecentlyUsedImages := []*ImageState{
		imageStateA, imageStateB, imageStateC,
	}
	leastRecentlyUsedImages := imageManager.(*dockerImageManager).getLeastRecentlyUsedImages(candidateImagesForDeletion)
	for i := range leastRecentlyUsedImages {
		if !reflect.DeepEqual(leastRecentlyUsedImages[i], expectedLeastRecentlyUsedImages[i]) {
			t.Error("Incorrect order of least recently used images")
		}
	}
}

func TestRemoveAlreadyExistingImageNameWithDifferentID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
	imageInspected := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil)
	err := imageManager.AddContainerReferenceToImageState(container)
	if err != nil {
		t.Error("Error in adding container to an existing image state")
	}
	container1 := &api.Container{
		Name:  "testContainer1",
		Image: "testContainerImage",
	}
	imageInspected1 := &docker.Image{
		ID: "sha256:asdfg",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected1, nil)
	err = imageManager.AddContainerReferenceToImageState(container1)
	if err != nil {
		t.Error("Error in adding container to an existing image state")
	}
	imageState, ok := imageManager.(*dockerImageManager).getImageState(imageInspected.ID)
	if !ok {
		t.Error("Error in retrieving existing Image State for the Container")
	}
	if len(imageState.Image.Names) != 0 {
		t.Error("Error in removing already existing image name with different ID")
	}
}

func TestImageCleanupHappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
	imageInspected := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil).AnyTimes()
	err := imageManager.AddContainerReferenceToImageState(container)
	if err != nil {
		t.Error("Error in adding container to an existing image state")
	}

	err = imageManager.RemoveContainerReferenceFromImageState(container)
	if err != nil {
		t.Error("Error removing container reference from image state")
	}

	imageState, _ := imageManager.(*dockerImageManager).getImageState(imageInspected.ID)
	imageState.PulledAt = time.Now().AddDate(0, -2, 0)
	imageState.LastUsedAt = time.Now().AddDate(0, -2, 0)

	ctx := context.Background()
	client.EXPECT().RemoveImage(container.Image).Return(nil)
	go imageManager.(*dockerImageManager).performPeriodicImageCleanup(ctx, 30*time.Second)
	time.Sleep(45 * time.Second)
	ctx.Done()
	if len(imageState.Image.Names) != 0 {
		t.Error("Error removing image name from state after the image is removed")
	}
	if len(imageManager.(*dockerImageManager).imageStates) != 0 {
		t.Error("Error removing image state after the image is removed")
	}
}

func TestImageCleanupCannotRemoveImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
	imageInspected := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil).AnyTimes()
	err := imageManager.AddContainerReferenceToImageState(container)
	if err != nil {
		t.Error("Error in adding container to an existing image state")
	}

	err = imageManager.RemoveContainerReferenceFromImageState(container)
	if err != nil {
		t.Error("Error removing container reference from image state")
	}

	imageState, _ := imageManager.(*dockerImageManager).getImageState(imageInspected.ID)
	imageState.PulledAt = time.Now().AddDate(0, -2, 0)
	imageState.LastUsedAt = time.Now().AddDate(0, -2, 0)

	ctx := context.Background()
	client.EXPECT().RemoveImage(container.Image).Return(errors.New("error removing image"))
	go imageManager.(*dockerImageManager).performPeriodicImageCleanup(ctx, 30*time.Second)
	time.Sleep(45 * time.Second)
	ctx.Done()
	if len(imageState.Image.Names) == 0 {
		t.Error("Error: image name should not be removed")
	}
	if len(imageManager.(*dockerImageManager).imageStates) == 0 {
		t.Error("Error: image state should not be removed")
	}
}

func TestImageCleanupRemoveImageById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
	imageInspected := &docker.Image{
		ID: "sha256:qwerty",
	}
	client.EXPECT().InspectImage(container.Image).Return(imageInspected, nil).AnyTimes()
	err := imageManager.AddContainerReferenceToImageState(container)
	if err != nil {
		t.Error("Error in adding container to an existing image state")
	}

	err = imageManager.RemoveContainerReferenceFromImageState(container)
	if err != nil {
		t.Error("Error removing container reference from image state")
	}

	imageState, _ := imageManager.(*dockerImageManager).getImageState(imageInspected.ID)
	imageManager.(*dockerImageManager).removeImageName(container.Image, imageState)
	imageState.PulledAt = time.Now().AddDate(0, -2, 0)
	imageState.LastUsedAt = time.Now().AddDate(0, -2, 0)

	ctx := context.Background()
	client.EXPECT().RemoveImage(sourceImage.ImageId).Return(nil)
	go imageManager.(*dockerImageManager).performPeriodicImageCleanup(ctx, 30*time.Second)
	time.Sleep(45 * time.Second)
	ctx.Done()
	if len(imageManager.(*dockerImageManager).imageStates) != 0 {
		t.Error("Error removing image state after the image is removed")
	}
}
