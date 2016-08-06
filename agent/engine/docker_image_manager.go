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
	"github.com/aws/amazon-ecs-agent/agent/engine/image"
	"github.com/cihub/seelog"
	"golang.org/x/net/context"
)

const (
	numImagesToDelete         = 5
	minimumAgeBeforeDeletion  = 1 * time.Hour
	imageCleanupTimeInterval  = 3 * time.Hour
	imageNotFoundErrorMessage = "no such image"
)

// ImageManager is responsible for saving the Image states,
// adding and removing container references to ImageStates
type ImageManager interface {
	AddContainerReferenceToImageState(container *api.Container) error
	RemoveContainerReferenceFromImageState(container *api.Container) error
	AddAllImageStates(imageStates []*image.ImageState)
	GetImageStateFromImageName(containerImageName string) *image.ImageState
	StartImageCleanupProcess(ctx context.Context, taskEngine TaskEngine)
}

// dockerImageManager accounts all the images and their states in the instance.
// It also has the cleanup policy configuration.
type dockerImageManager struct {
	imageStates []*image.ImageState
	client      DockerClient
	// coarse grained lock for updating container references as part of image states
	updateLock         sync.RWMutex
	imageCleanupTicker *time.Ticker
}

// ImageStatesForDeletion is used for implementing the sort interface
type ImageStatesForDeletion []*image.ImageState

func NewImageManager(client DockerClient) ImageManager {
	return &dockerImageManager{
		client: client,
	}
}

func (imageManager *dockerImageManager) addImageState(imageState *image.ImageState) {
	imageManager.imageStates = append(imageManager.imageStates, imageState)
}

func (imageManager *dockerImageManager) AddAllImageStates(imageStates []*image.ImageState) {
	imageManager.updateLock.Lock()
	defer imageManager.updateLock.Unlock()
	for _, imageState := range imageStates {
		imageManager.addImageState(imageState)
	}
}

// getAllImageStates returns the list of imageStates in the instance
func (imageManager *dockerImageManager) getAllImageStates() []*image.ImageState {
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

	imageManager.removeExistingImageNameOfDifferentID(container.Image, imageInspected.ID)
	imageState, ok := imageManager.getImageState(imageInspected.ID)
	if ok {
		// Image State already exists; add Container to it
		exists := imageManager.hasImageName(imageState, container.Image)
		if !exists {
			seelog.Infof("Adding image name- %v to Image state- %v", container.Image, imageState.Image.ImageId)
			imageManager.addImageName(imageState, container.Image)
		}
		seelog.Infof("Updating container reference- %v in Image state- %v", container.Name, imageState.Image.ImageId)
		imageManager.updateContainerReference(imageState, container)
		return nil
	}
	sourceImage := &image.Image{
		ImageId: imageInspected.ID,
		Size:    imageInspected.Size,
	}
	sourceImageState := &image.ImageState{
		Image:    sourceImage,
		PulledAt: time.Now(),
	}
	seelog.Infof("Adding image name- %v to Image state- %v", container.Image, sourceImageState.Image.ImageId)
	imageManager.addImageName(sourceImageState, container.Image)
	seelog.Infof("Updating container reference- %v in Image state %v", container.Name, sourceImage.ImageId)
	imageManager.updateContainerReference(sourceImageState, container)
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
func (imageManager *dockerImageManager) getImageState(containerImageID string) (*image.ImageState, bool) {
	for _, imageState := range imageManager.getAllImageStates() {
		if imageState.Image.ImageId == containerImageID {
			return imageState, true
		}
	}
	return nil, false
}

func (imageManager *dockerImageManager) hasImageName(imageState *image.ImageState, containerImageName string) bool {
	for _, imageName := range imageState.Image.Names {
		if imageName == containerImageName {
			return true
		}
	}
	return false
}

func (imageManager *dockerImageManager) updateContainerReference(imageState *image.ImageState, container *api.Container) {
	imageState.Containers = append(imageState.Containers, container)
}

func (imageManager *dockerImageManager) addImageName(imageState *image.ImageState, imageName string) {
	imageState.Image.Names = append(imageState.Image.Names, imageName)
}

// removeImageState removes the imageState from the list of imageState objects in ImageManager
func (imageManager *dockerImageManager) removeImageState(imageStateToBeRemoved *image.ImageState) {
	for i, imageState := range imageManager.imageStates {
		if imageState.Image.ImageId == imageStateToBeRemoved.Image.ImageId {
			// Image State found; hence remove it
			seelog.Infof("Removing Image State: %v from Image Manager", imageState.Image.ImageId)
			imageManager.imageStates = append(imageManager.imageStates[:i], imageManager.imageStates[i+1:]...)
			return
		}
	}
}

func (imageManager *dockerImageManager) getCandidateImagesForDeletion() []*image.ImageState {
	imageStates := imageManager.getAllImageStates()
	if len(imageStates) < 1 {
		// no image states present in image manager
		return nil
	}
	var imagesForDeletion []*image.ImageState
	for _, imageState := range imageStates {
		if imageManager.isImageOldEnough(imageState) && imageManager.hasNoAssociatedContainers(imageState) {
			seelog.Infof("Candidate image for deletion: %+v", imageState)
			imagesForDeletion = append(imagesForDeletion, imageState)
		}
	}
	return imagesForDeletion
}

func (imageManager *dockerImageManager) isImageOldEnough(imageState *image.ImageState) bool {
	ageOfImage := time.Now().Sub(imageState.PulledAt)
	return ageOfImage > minimumAgeBeforeDeletion
}

func (imageManager *dockerImageManager) hasNoAssociatedContainers(imageState *image.ImageState) bool {
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

func (imageManager *dockerImageManager) getLeastRecentlyUsedImages(imagesForDeletion []*image.ImageState) []*image.ImageState {
	var candidateImages ImageStatesForDeletion
	for _, imageState := range imagesForDeletion {
		candidateImages = append(candidateImages, imageState)
	}
	// sort images in the order of last used times
	sort.Sort(candidateImages)
	var lruImages []*image.ImageState
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
		for i, _ := range imageState.Image.Names {
			if imageState.Image.Names[i] == containerImageName && imageState.Image.ImageId != inspectedImageID {
				// image with same name pulled in the instance. Untag the already existing image name
				seelog.Infof("Removing Image Name: %v from Image- %v", containerImageName, imageState.Image.ImageId)
				imageState.Image.Names = append(imageState.Image.Names[:i], imageState.Image.Names[i+1:]...)
				return
			}
		}
	}
}

func (imageManager *dockerImageManager) StartImageCleanupProcess(ctx context.Context, taskEngine TaskEngine) {
	// passing the cleanup interval as argument which would help during testing
	imageManager.performPeriodicImageCleanup(ctx, taskEngine, imageCleanupTimeInterval)
}

func (imageManager *dockerImageManager) performPeriodicImageCleanup(ctx context.Context, taskEngine TaskEngine, imageCleanupInterval time.Duration) {
	imageManager.imageCleanupTicker = time.NewTicker(imageCleanupInterval)
	for {
		select {
		case <-imageManager.imageCleanupTicker.C:
			go imageManager.removeUnusedImages(taskEngine)
		case <-ctx.Done():
			imageManager.imageCleanupTicker.Stop()
			return
		}
	}
}

func (imageManager *dockerImageManager) removeUnusedImages(taskEngine TaskEngine) {
	// coarse grained lock for not letting other events change the Image Manager while cleanup
	imageManager.updateLock.Lock()
	defer imageManager.updateLock.Unlock()
	candidateImageStatesForDeletion := imageManager.getCandidateImagesForDeletion()
	if len(candidateImageStatesForDeletion) < 1 {
		seelog.Infof("No eligible images for deletion for this cleanup cycle")
		return
	}
	leastRecentlyUsedImages := imageManager.getLeastRecentlyUsedImages(candidateImageStatesForDeletion)
	imageManager.removeImages(taskEngine, leastRecentlyUsedImages)
}

func (imageManager *dockerImageManager) removeImages(taskEngine TaskEngine, leastRecentlyUsedImages []*image.ImageState) {
	if len(leastRecentlyUsedImages) < 1 {
		seelog.Infof("No images returned for deletion")
		return
	}
	for _, leastRecentlyUsedImage := range leastRecentlyUsedImages {
		if len(leastRecentlyUsedImage.Image.Names) == 0 {
			// potentially untagged image of format <none>:<none>; remove by ID
			imageManager.deleteImage(taskEngine, leastRecentlyUsedImage.Image.ImageId, leastRecentlyUsedImage)
		} else {
			// Image has multiple tags/repos. Untag each name and delete the final reference to image
			imageNames := leastRecentlyUsedImage.Image.Names
			for _, imageName := range imageNames {
				imageManager.deleteImage(taskEngine, imageName, leastRecentlyUsedImage)
			}
		}
	}
}

func (imageManager *dockerImageManager) deleteImage(taskEngine TaskEngine, imageIdentity string, imageState *image.ImageState) {
	err := imageManager.client.RemoveImage(imageIdentity, removeImageTimeout)
	if err != nil {
		if err.Error() == imageNotFoundErrorMessage {
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
		taskEngine.(*DockerTaskEngine).state.RemoveImageState(imageState)
		taskEngine.(*DockerTaskEngine).saver.Save()
	}
}

func (imageManager *dockerImageManager) removeImageName(containerImageName string, imageState *image.ImageState) {
	for i, imageName := range imageState.Image.Names {
		if imageName == containerImageName {
			imageState.Image.Names = append(imageState.Image.Names[:i], imageState.Image.Names[i+1:]...)
		}
	}
}

func (imageManager *dockerImageManager) GetImageStateFromImageName(containerImageName string) *image.ImageState {
	imageManager.updateLock.Lock()
	defer imageManager.updateLock.Unlock()
	for _, imageState := range imageManager.getAllImageStates() {
		for _, imageName := range imageState.Image.Names {
			if imageName == containerImageName {
				return imageState
			}
		}
	}
	return nil
}
