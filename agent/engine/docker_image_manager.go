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
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aws/amazon-ecs-agent/agent/api"
	"github.com/cihub/seelog"
)

const (
	numImagesToDelete        = 5
	minimumAgeBeforeDeletion = 1 * time.Hour
)

// ImageManager is responsible for saving the Image states,
// adding and removing container references to ImageStates
type ImageManager interface {
	AddContainerReferenceToImageState(container *api.Container) error
	RemoveContainerReferenceFromImageState(container *api.Container) error
}

// Image represents a docker image and its various properties
type Image struct {
	ImageId string
	Names   []string
	Size    int64
	// TODO: fine grained locking for appending/retrieving image names in corresponding images
	// imageLock sync.RWMutex
}

// ImageState represents a docker image
// and its state information such as containers associated with it
type ImageState struct {
	Image      *Image
	Containers []*api.Container
	PulledAt   time.Time
	LastUsedAt time.Time
	// TODO: fine grained locking for updating/retrieving container references in image states
	// containerLock sync.RWMutex
}

// dockerImageManager accounts all the images and their states in the instance.
// It also has the cleanup policy configuration.
type dockerImageManager struct {
	imageStates []*ImageState
	// TODO: fine grained locking for updating/retrieving image states in docker image manager
	// imageStateLock sync.RWMutex
	client DockerClient
	// coarse grained lock for updating container references as part of image states
	updateLock sync.RWMutex
}

// ImageStatesForDeletion is used for implementing the sort interface
type ImageStatesForDeletion []*ImageState

func NewImageManager(client DockerClient) ImageManager {
	return &dockerImageManager{
		client: client,
	}
}

// addImageState appends the imageState to list of imageState objects in ImageManager
func (imageManager *dockerImageManager) addImageState(imageState *ImageState) {
	// TODO: fine grained locking for appending image state to image manager
	// imageManager.imageStateLock.Lock()
	// defer imageManager.imageStateLock.Unlock()
	imageManager.imageStates = append(imageManager.imageStates, imageState)
}

// getAllImageStates returns the list of imageStates in the instance
func (imageManager *dockerImageManager) getAllImageStates() []*ImageState {
	// TODO: fine grained locking for retrieving image states from image manager
	// imageManager.imageStateLock.RLock()
	// defer imageManager.imageStateLock.RUnlock()
	return imageManager.imageStates
}

// AddContainerReferenceToImageState adds container reference to the corresponding imageState object
func (imageManager *dockerImageManager) AddContainerReferenceToImageState(container *api.Container) error {
	imageManager.updateLock.Lock()
	defer imageManager.updateLock.Unlock()
	if container.Image == "" {
		return fmt.Errorf("Invalid container reference: Empty image name")
	}
	// Inspect image for obtaining Container's Image ID
	imageInspected, err := imageManager.client.InspectImage(container.Image)
	if err != nil {
		seelog.Errorf("Error inspecting image %v: %v", container.Image, err)
		return err
	}

	imageState, ok := imageManager.getImageState(imageInspected.ID)

	if ok {
		// Image State already exists; add Container to it
		exists := imageState.hasImageName(container.Image)
		if !exists {
			seelog.Infof("Adding image name- %v to Image state- %v", container.Image, imageState.Image.ImageId)
			imageState.addImageName(container.Image)
		}
		seelog.Infof("Updating container reference- %v in Image state- %v", container.Name, imageState.Image.ImageId)
		imageState.updateContainerReference(container)
		return nil
	}
	sourceImage := &Image{
		ImageId: imageInspected.ID,
		Size:    imageInspected.Size,
	}
	sourceImageState := &ImageState{
		Image:    sourceImage,
		PulledAt: time.Now(),
	}
	seelog.Infof("Adding image name- %v to Image state- %v", container.Image, sourceImageState.Image.ImageId)
	sourceImageState.addImageName(container.Image)
	seelog.Infof("Updating container reference- %v in Image state %v", container.Name, sourceImage.ImageId)
	sourceImageState.updateContainerReference(container)
	imageManager.addImageState(sourceImageState)
	return nil
}

// RemoveContainerReferenceFromImageState removes container reference from the corresponding imageState object
func (imageManager *dockerImageManager) RemoveContainerReferenceFromImageState(container *api.Container) error {
	imageManager.updateLock.Lock()
	defer imageManager.updateLock.Unlock()
	if container.Image == "" {
		return fmt.Errorf("Invalid container reference: Empty image name")
	}
	// Inspect image for obtaining Container's Image ID
	imageInspected, err := imageManager.client.InspectImage(container.Image)
	if err != nil {
		seelog.Errorf("Error inspecting image %v: %v", container.Image, err)
		return err
	}

	// Find image state that this container is part of, and remove the reference
	imageState, ok := imageManager.getImageState(imageInspected.ID)
	if !ok {
		return fmt.Errorf("Cannot find image state for the container to be removed")
	}
	// Found matching ImageState
	// TODO: fine grained locking for retrieving container references in an image state
	// imageState.containerLock.Lock()
	// defer imageState.containerLock.Unlock()
	for i, _ := range imageState.Containers {
		if imageState.Containers[i].Name == container.Name {
			// Container reference found; hence remove it
			seelog.Infof("Removing Container Reference: %v from Image State- %v", container.Name, imageState.Image.ImageId)
			imageState.Containers = append(imageState.Containers[:i], imageState.Containers[i+1:]...)
			// Update the last used time for the image
			imageState.LastUsedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("Container reference is not found in the image state")
}

// getImageState returns the ImageState object that the container is referenced at
func (imageManager *dockerImageManager) getImageState(containerImageID string) (*ImageState, bool) {
	for _, imageState := range imageManager.getAllImageStates() {
		if imageState.Image.ImageId == containerImageID {
			return imageState, true
		}
	}
	return nil, false
}

func (imageState *ImageState) hasImageName(containerImageName string) bool {
	// TODO: fine grained locking for retrieving image names from the image state
	// imageState.Image.imageLock.RLock()
	// defer imageState.Image.imageLock.RUnlock()
	for _, imageName := range imageState.Image.Names {
		if imageName == containerImageName {
			return true
		}
	}
	return false
}

func (imageState *ImageState) updateContainerReference(container *api.Container) {
	// TODO: fine grained locking for appending container reference to image state
	// imageState.containerLock.Lock()
	// defer imageState.containerLock.Unlock()
	imageState.Containers = append(imageState.Containers, container)
}

func (imageState *ImageState) addImageName(imageName string) {
	// TODO: fine grained locking for adding image name to the image state
	// imageState.Image.imageLock.Lock()
	// defer imageState.Image.imageLock.Unlock()
	imageState.Image.Names = append(imageState.Image.Names, imageName)
}

func (imageManager *dockerImageManager) getCandidateImagesForDeletion() []*ImageState {
	// TODO: Lock to be used for imageStates variable
	imageStates := imageManager.getAllImageStates()
	if len(imageStates) < 1 {
		// no image states present in image manager
		return nil
	}
	var imagesForDeletion []*ImageState
	for _, imageState := range imageStates {
		if imageManager.isImageOldEnough(imageState) && imageState.hasNoAssociatedContainers() {
			seelog.Infof("Candidate image for deletion: %+v", imageState)
			imagesForDeletion = append(imagesForDeletion, imageState)
		}
	}
	return imagesForDeletion
}

func (imageManager *dockerImageManager) isImageOldEnough(imageState *ImageState) bool {
	ageOfImage := time.Now().Sub(imageState.PulledAt)
	return ageOfImage > minimumAgeBeforeDeletion
}

func (imageState *ImageState) hasNoAssociatedContainers() bool {
	// TODO: fine grained locking for retrieving container references in the image state
	return len(imageState.Containers) == 0
}

// Implementing sort interface based on last used times of the images
func (imageStates ImageStatesForDeletion) Len() int {
	return len(imageStates)
}

func (imageStates ImageStatesForDeletion) Less(i, j int) bool {
	return imageStates[i].LastUsedAt.Before(imageStates[j].LastUsedAt)
}

func (imageStates ImageStatesForDeletion) Swap(i, j int) {
	imageStates[i], imageStates[j] = imageStates[j], imageStates[i]
}

func (imageManager *dockerImageManager) getLeastRecentlyUsedImages(imagesForDeletion []*ImageState) []*ImageState {
	var candidateImages ImageStatesForDeletion
	for _, imageState := range imagesForDeletion {
		candidateImages = append(candidateImages, imageState)
	}
	// sort images in the order of last used times
	sort.Sort(candidateImages)
	var lruImages []*ImageState
	for _, lruImage := range candidateImages {
		lruImages = append(lruImages, lruImage)
	}
	if len(lruImages) <= numImagesToDelete {
		return lruImages
	}
	return lruImages[:numImagesToDelete]
}
