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
	"github.com/aws/amazon-ecs-agent/agent/config"
	"github.com/aws/amazon-ecs-agent/agent/ec2"
	"github.com/aws/amazon-ecs-agent/agent/engine/dockerclient"
	"github.com/aws/amazon-ecs-agent/agent/engine/dockerstate"
	"github.com/aws/amazon-ecs-agent/agent/engine/image"
	"github.com/aws/amazon-ecs-agent/agent/statemanager"
	"github.com/cihub/seelog"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/mock/gomock"
	"golang.org/x/net/context"
)

func TestAddAndRemoveContainerToImageStateReferenceHappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &image.Image{
		ImageId: "sha256:qwerty",
	}
	sourceImageState := &image.ImageState{
		Image:    sourceImage,
		PulledAt: time.Now().AddDate(0, -2, 0),
	}
	sourceImageState.AddImageName(container.Image)
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &image.Image{
		ImageId: "sha256:qwerty",
	}
	sourceImageState := &image.ImageState{
		Image:    sourceImage,
		PulledAt: time.Now(),
	}
	sourceImageState.AddImageName(container.Image)
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &image.Image{
		ImageId: "sha256:qwerty",
	}
	sourceImageState := &image.ImageState{
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &image.Image{
		ImageId: "sha256:qwerty",
	}
	sourceImageState := &image.ImageState{
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	imageStates := imageManager.(*dockerImageManager).getCandidateImagesForDeletion()
	if imageStates != nil {
		t.Error("Expected no image state to be returned for deletion")
	}
}

func TestGetCandidateImagesForDeletionImageJustPulled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := NewMockDockerClient(ctrl)
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	sourceImage := &image.Image{}
	sourceImageState := &image.ImageState{
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &image.Image{
		ImageId: "sha256:qwerty",
	}
	sourceImage.Names = append(sourceImage.Names, container.Image)
	sourceImageState := &image.ImageState{
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	container2 := &api.Container{
		Name:  "testContainer2",
		Image: "testContainerImage",
	}
	sourceImage := &image.Image{
		ImageId: "sha256:qwerty",
	}
	sourceImage.Names = append(sourceImage.Names, container.Image)
	sourceImageState := &image.ImageState{
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	imageStateA := &image.ImageState{
		LastUsedAt: time.Now().AddDate(0, -5, 0),
	}
	imageStateB := &image.ImageState{
		LastUsedAt: time.Now().AddDate(0, -3, 0),
	}
	imageStateC := &image.ImageState{
		LastUsedAt: time.Now().AddDate(0, -2, 0),
	}
	imageStateD := &image.ImageState{
		LastUsedAt: time.Now().AddDate(0, -6, 0),
	}
	imageStateE := &image.ImageState{
		LastUsedAt: time.Now().AddDate(0, -4, 0),
	}
	imageStateF := &image.ImageState{
		LastUsedAt: time.Now().AddDate(0, -1, 0),
	}

	candidateImagesForDeletion := []*image.ImageState{
		imageStateA, imageStateB, imageStateC, imageStateD, imageStateE, imageStateF,
	}
	expectedLeastRecentlyUsedImages := []*image.ImageState{
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	imageStateA := &image.ImageState{
		LastUsedAt: time.Now().AddDate(0, -5, 0),
	}
	imageStateB := &image.ImageState{
		LastUsedAt: time.Now().AddDate(0, -3, 0),
	}
	imageStateC := &image.ImageState{
		LastUsedAt: time.Now().AddDate(0, -2, 0),
	}
	candidateImagesForDeletion := []*image.ImageState{
		imageStateA, imageStateB, imageStateC,
	}
	expectedLeastRecentlyUsedImages := []*image.ImageState{
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &image.Image{
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	//taskEngine := NewTaskEngine(&config.Config{}, nil, nil, nil)
	imageManager.SetSaver(statemanager.NewNoopStateManager())
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &image.Image{
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

	client.EXPECT().RemoveImage(container.Image, removeImageTimeout).Return(nil)
	parent := context.Background()
	ctx, cancel := context.WithCancel(parent)
	go imageManager.(*dockerImageManager).performPeriodicImageCleanup(ctx, 2*time.Millisecond)
	time.Sleep(3 * time.Millisecond)
	cancel()
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	imageManager.SetSaver(statemanager.NewNoopStateManager())
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &image.Image{
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

	client.EXPECT().RemoveImage(container.Image, removeImageTimeout).Return(errors.New("error removing image"))
	imageManager.(*dockerImageManager).removeUnusedImages()
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
	imageManager := NewImageManager(client, dockerstate.NewDockerTaskEngineState())
	imageManager.SetSaver(statemanager.NewNoopStateManager())
	container := &api.Container{
		Name:  "testContainer",
		Image: "testContainerImage",
	}
	sourceImage := &image.Image{
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
	imageState.RemoveImageName(container.Image)
	imageState.PulledAt = time.Now().AddDate(0, -2, 0)
	imageState.LastUsedAt = time.Now().AddDate(0, -2, 0)

	client.EXPECT().RemoveImage(sourceImage.ImageId, removeImageTimeout).Return(nil)
	imageManager.(*dockerImageManager).removeUnusedImages()
	if len(imageManager.(*dockerImageManager).imageStates) != 0 {
		t.Error("Error removing image state after the image is removed")
	}
}

func TestPeriodicImageCleanupCheck(t *testing.T) {
	cfg, _ := config.NewConfig(ec2.NewBlackholeEC2MetadataClient())
	clientFactory := dockerclient.NewFactory("unix:///var/run/docker.sock")
	dockerClient, err := NewDockerGoClient(clientFactory, false, cfg)
	if err != nil {
		t.Errorf("Error creating Docker client: %v", err)
	}

	imageManager := NewImageManager(dockerClient, dockerstate.NewDockerTaskEngineState())
	imageManager.SetSaver(statemanager.NewNoopStateManager())

	container := &api.Container{
		Name:  "testContainer1",
		Image: "ruby:latest",
	}

	container1 := &api.Container{
		Name:  "testContainer2",
		Image: "ubuntu:latest",
	}

	container2 := &api.Container{
		Name:  "testContainer3",
		Image: "myruby:latest",
	}

	if len(imageManager.(*dockerImageManager).imageStates) != 0 {
		t.Error("Somehow extra image states there")
	}
	err1 := imageManager.AddContainerReferenceToImageState(container)
	if err1 != nil {
		t.Error("Error in adding container to an existing image state")
	}
	err1 = imageManager.AddContainerReferenceToImageState(container1)
	if err1 != nil {
		t.Error("Error in adding container to an existing image state")
	}
	err1 = imageManager.AddContainerReferenceToImageState(container2)
	if err1 != nil {
		t.Error("Error in adding container to an existing image state")
	}

	err2 := imageManager.RemoveContainerReferenceFromImageState(container)
	if err2 != nil {
		t.Error("Error removing container reference from image state")
	}
	err2 = imageManager.RemoveContainerReferenceFromImageState(container1)
	if err2 != nil {
		t.Error("Error removing container reference from image state")
	}
	err2 = imageManager.RemoveContainerReferenceFromImageState(container2)
	if err2 != nil {
		t.Error("Error removing container reference from image state")
	}

	imageState, _ := imageManager.(*dockerImageManager).getImageState("sha256:7b66156f376cb0f12f297ce24415d7e4eabc3c453152dd4559d1bea62646ecf5")
	if len(imageState.Containers) != 0 {
		t.Error("Error removing container reference from image state")
	}
	imageState.PulledAt = time.Now().AddDate(0, -2, 0)
	imageState.LastUsedAt = time.Now().AddDate(0, -2, 0)

	imageState, _ = imageManager.(*dockerImageManager).getImageState("sha256:42118e3df429f09ca581a9deb3df274601930e428e452f7e4e9f1833c56a100a")
	if len(imageState.Containers) != 0 {
		t.Error("Error removing container reference from image state")
	}
	imageState.PulledAt = time.Now().AddDate(0, -3, 0)
	imageState.LastUsedAt = time.Now().AddDate(0, -3, 0)

	seelog.Infof("Image states as part of Image Manager:")

	for _, imageStateObtained := range imageManager.(*dockerImageManager).getAllImageStates() {
		seelog.Infof("Image state: %+v", imageStateObtained)
		seelog.Infof("Images:")
		for _, imageName := range imageStateObtained.Image.Names {
			seelog.Infof("%v", imageName)
		}
		seelog.Infof("------------------")
	}
	//ctx := context.Background()
	//imageManager.(*dockerImageManager).performPeriodicImageCleanup(ctx, 30*time.Second)
	imageManager.(*dockerImageManager).removeUnusedImages()
	time.Sleep(30 * time.Second)
}
