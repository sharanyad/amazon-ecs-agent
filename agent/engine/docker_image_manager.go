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
	"golang.org/x/net/context"
)

const (
	numImagesToDelete             = 5
	minimumAgeBeforeDeletion      = 1 * time.Hour
	imageCleanupTimeInterval      = 3 * time.Hour
	imageNotFoundForDeletionError = "no such image"
)

// ImageManager is responsible for saving the Image states,
// adding and removing container references to ImageStates
type ImageManager interface {
	AddContainerReferenceToImageState(container *api.Container) error
	RemoveContainerReferenceFromImageState(container *api.Container) error
	StartImageCleanupProcess(ctx context.Context)
}

// Image represents a docker image and its various properties
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
	updateLock sync.RWMutex
}

// dockerImageManager accounts all the images and their states in the instance.
// It also has the cleanup policy configuration.
type dockerImageManager struct {
	imageStates []*ImageState
	client      DockerClient
	// coarse grained lock for updating container references as part of image states
	updateLock         sync.RWMutex
	imageCleanupTicker *time.Ticker
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
	imageManager.imageStates = append(imageManager.imageStates, imageState)
}

// removeImageState removes the imageState from the list of imageState objects in ImageManager
func (imageManager *dockerImageManager) removeImageState(imageStateToBeRemoved *ImageState) {
	imageManager.updateLock.Lock()
	defer imageManager.updateLock.Unlock()
	for i, imageState := range imageManager.imageStates {
		if imageState.Image.ImageId == imageStateToBeRemoved.Image.ImageId {
			// Image State found; hence remove it
			seelog.Infof("Removing Image State: %v from Image Manager", imageState.Image.ImageId)
			imageManager.imageStates = append(imageManager.imageStates[:i], imageManager.imageStates[i+1:]...)
			return
		}
	}
}

// getAllImageStates returns the list of imageStates in the instance
func (imageManager *dockerImageManager) getAllImageStates() []*ImageState {
	return imageManager.imageStates
}

// AddContainerReferenceToImageState adds container reference to the corresponding imageState object
func (imageManager *dockerImageManager) AddContainerReferenceToImageState(container *api.Container) error {
	if container.Image == "" {
		return fmt.Errorf("Invalid container reference: Empty image name")
	}
	// Inspect image for obtaining Container's Image ID
	imageInspected, err := imageManager.client.InspectImage(container.Image)
	if err != nil {
		seelog.Errorf("Error inspecting image %v: %v", container.Image, err)
		return err
	}
	added := imageManager.addContainerReferenceToExistingImageState(container, imageInspected.ID)
	if !added {
		imageManager.addContainerReferenceToNewImageState(container, imageInspected.ID, imageInspected.Size)
	}
	return nil
}

func (imageManager *dockerImageManager) addContainerReferenceToExistingImageState(container *api.Container, imageId string) bool {
	// this lock is used for reading the image states in the image manager
	imageManager.updateLock.RLock()
	defer imageManager.updateLock.RUnlock()
	imageManager.removeExistingImageNameOfDifferentID(container.Image, imageId)
	imageState, ok := imageManager.getImageState(imageId)
	if ok {
		imageState.updateImageState(container)
	}
	return ok
}

func (imageManager *dockerImageManager) addContainerReferenceToNewImageState(container *api.Container, imageId string, imageSize int64) {
	// this lock is used while creating and adding new image state to image manager
	imageManager.updateLock.Lock()
	defer imageManager.updateLock.Unlock()
	// check to see if a different thread added image state for same image ID
	imageState, ok := imageManager.getImageState(imageId)
	if ok {
		imageState.updateImageState(container)
	} else {
		sourceImage := &Image{
			ImageId: imageId,
			Size:    imageSize,
		}
		sourceImageState := &ImageState{
			Image:    sourceImage,
			PulledAt: time.Now(),
		}
		sourceImageState.updateImageState(container)
		imageManager.addImageState(sourceImageState)
	}
}

func (imageState *ImageState) updateImageState(container *api.Container) {
	imageState.addImageName(container.Image)
	imageState.updateContainerReference(container)
}

// RemoveContainerReferenceFromImageState removes container reference from the corresponding imageState object
func (imageManager *dockerImageManager) RemoveContainerReferenceFromImageState(container *api.Container) error {
	// this lock is for reading image states and finding the one that the container belongs to
	imageManager.updateLock.RLock()
	defer imageManager.updateLock.RUnlock()
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
	// Get the image state write lock for updating container reference
	imageState.updateLock.Lock()
	defer imageState.updateLock.Unlock()
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
	for _, imageName := range imageState.Image.Names {
		if imageName == containerImageName {
			return true
		}
	}
	return false
}

func (imageState *ImageState) updateContainerReference(container *api.Container) {
	imageState.updateLock.Lock()
	defer imageState.updateLock.Unlock()
	seelog.Infof("Updating container reference- %v in Image state- %v", container.Name, imageState.Image.ImageId)
	imageState.Containers = append(imageState.Containers, container)
	imageState.LastUsedAt = time.Now()
}

func (imageState *ImageState) addImageName(imageName string) {
	imageState.updateLock.Lock()
	defer imageState.updateLock.Unlock()
	if !imageState.hasImageName(imageName) {
		seelog.Infof("Adding image name- %v to Image state- %v", imageName, imageState.Image.ImageId)
		imageState.Image.Names = append(imageState.Image.Names, imageName)
	}
}

func (imageManager *dockerImageManager) getCandidateImagesForDeletion() []*ImageState {
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

func (imageManager *dockerImageManager) removeExistingImageNameOfDifferentID(containerImageName string, inspectedImageID string) {
	for _, imageState := range imageManager.getAllImageStates() {
		// image with same name pulled in the instance. Untag the already existing image name
		if imageState.Image.ImageId != inspectedImageID {
			imageManager.removeImageName(containerImageName, imageState)
		}
	}
}

func (imageManager *dockerImageManager) StartImageCleanupProcess(ctx context.Context) {
	// passing the cleanup interval as argument which would help during testing
	imageManager.performPeriodicImageCleanup(ctx, imageCleanupTimeInterval)
}

func (imageManager *dockerImageManager) performPeriodicImageCleanup(ctx context.Context, imageCleanupInterval time.Duration) {
	imageManager.imageCleanupTicker = time.NewTicker(imageCleanupInterval)
	for {
		select {
		case <-imageManager.imageCleanupTicker.C:
			go imageManager.removeUnusedImages()
		case <-ctx.Done():
			imageManager.imageCleanupTicker.Stop()
			return
		}
	}
}

func (imageManager *dockerImageManager) removeUnusedImages() {
	leastRecentlyUsedImages := imageManager.getUnusedImagesForDeletion()
	imageManager.removeImages(leastRecentlyUsedImages)
}

func (imageManager *dockerImageManager) getUnusedImagesForDeletion() []*ImageState {
	imageManager.updateLock.RLock()
	defer imageManager.updateLock.RUnlock()
	candidateImageStatesForDeletion := imageManager.getCandidateImagesForDeletion()
	if len(candidateImageStatesForDeletion) < 1 {
		seelog.Infof("No eligible images for deletion for this cleanup cycle")
		return nil
	}
	return imageManager.getLeastRecentlyUsedImages(candidateImageStatesForDeletion)
}

func (imageManager *dockerImageManager) removeImages(leastRecentlyUsedImages []*ImageState) {
	// remove an image either by ID or names
	if len(leastRecentlyUsedImages) < 1 {
		seelog.Infof("No images returned for deletion")
		return
	}
	for _, leastRecentlyUsedImage := range leastRecentlyUsedImages {
		imageNames := leastRecentlyUsedImage.Image.Names
		if len(imageNames) == 0 {
			// potentially untagged image of format <none>:<none>; remove by ID
			imageManager.deleteImage(leastRecentlyUsedImage.Image.ImageId, leastRecentlyUsedImage)
		} else {
			// Image has multiple tags/repos. Untag each name and delete the final reference to image
			for _, imageName := range imageNames {
				imageManager.deleteImage(imageName, leastRecentlyUsedImage)
			}
		}
	}
}

func (imageManager *dockerImageManager) deleteImage(imageIdentity string, imageState *ImageState) {
	err := imageManager.client.RemoveImage(imageIdentity, removeImageTimeout)
	if err != nil {
		if err.Error() == imageNotFoundForDeletionError {
			seelog.Errorf("Image already removed from the instance")
		} else {
			seelog.Errorf("Error removing Image %v - %v", imageIdentity, err)
			return
		}
	}
	seelog.Infof("Image removed: %v", imageIdentity)
	imageManager.removeImageName(imageIdentity, imageState)
	if len(imageState.Image.Names) == 0 {
		imageManager.removeImageState(imageState)
	}
}

func (imageManager *dockerImageManager) removeImageName(containerImageName string, imageState *ImageState) {
	imageState.updateLock.Lock()
	defer imageState.updateLock.Unlock()
	for i, imageName := range imageState.Image.Names {
		if imageName == containerImageName {
			imageState.Image.Names = append(imageState.Image.Names[:i], imageState.Image.Names[i+1:]...)
		}
	}
}
