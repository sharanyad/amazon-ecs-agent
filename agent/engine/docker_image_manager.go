package engine

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aws/amazon-ecs-agent/agent/api"
	"github.com/cihub/seelog"
	docker "github.com/fsouza/go-dockerclient"
)

const (
	IMAGES_TO_DELETE = 5
)

// ImageManager is responsible for saving the Image states,
// adding and removing container references to ImageStates
type ImageManager interface {
	AddImageState(imageState *ImageState)
	AddContainerReferenceToImageState(container *api.Container) error
	RemoveContainerReferenceFromImageState(container *api.Container) error
	// TODO: RemoveImageState(imageState *ImageState) error
	GetEligibleImagesForDeletion() []*ImageState
	GetLeastRecentlyUsedImages(ImageStatesForDeletion) ImageStatesForDeletion
}

// Image is the type representing a docker image and its various properties
type Image struct {
	ImageId   string
	Names     []string
	Repos     []string
	Tags      []string
	Size      int64
	imageLock sync.RWMutex
}

// ImageState is the type representing a docker image
// and its state information such as containers associated with it
type ImageState struct {
	Image        *Image
	Containers   []*api.Container
	PulledTime   time.Time
	LastUsedTime time.Time
	updateLock   sync.RWMutex
}

// dockerImageManager accounts all the images and their states in the instance.
// It also has the cleanup policy configuration.
type dockerImageManager struct {
	imageStates []*ImageState
	// TODO: add cleanup policy details
	imageStateLock sync.RWMutex
	client         DockerClient
	minAge         time.Duration
}

type ImageStatesForDeletion []*ImageState

func NewImageManager(client DockerClient) ImageManager {
	return &dockerImageManager{
		client: client,
		minAge: 1 * time.Hour,
	}
}

// AddImageState appends the imageState to list of imageState objects in ImageManager
func (imageManager *dockerImageManager) AddImageState(imageState *ImageState) {
	imageManager.imageStateLock.Lock()
	defer imageManager.imageStateLock.Unlock()
	imageManager.imageStates = append(imageManager.imageStates, imageState)
}

// getAllImageStates returns the list of imageStates in the instance
func (imageManager *dockerImageManager) getAllImageStates() []*ImageState {
	imageManager.imageStateLock.RLock()
	defer imageManager.imageStateLock.RUnlock()
	return imageManager.imageStates
}

// AddContainerReferenceToImageState adds container reference to the corresponding imageState object
func (imageManager *dockerImageManager) AddContainerReferenceToImageState(container *api.Container) error {
	if container.Image == "" {
		return fmt.Errorf("Invalid container reference: Empty image name")
	}
	imageState, ok := imageManager.getImageState(container)
	if ok {
		// Image State already exists; add Container to it
		seelog.Infof("Adding container reference- %v to Image state- %v", container.Name, imageState.Image.ImageId)
		ok = imageManager.hasImageName(imageState, container.Image)
		if !ok {
			seelog.Infof("Adding image name- %v to Image state- %v", container.Image, imageState.Image.ImageId)
			repository, tag := docker.ParseRepositoryTag(container.Image)
			imageState.Image.imageLock.Lock()
			defer imageState.Image.imageLock.Unlock()
			imageState.Image.Names = append(imageState.Image.Names, container.Image)
			imageState.Image.Repos = append(imageState.Image.Repos, repository)
			imageState.Image.Tags = append(imageState.Image.Tags, tag)
		}
		imageState.updateLock.Lock()
		defer imageState.updateLock.Unlock()
		imageState.Containers = append(imageState.Containers, container)
		return nil
	}
	// Inspect image for creating new Image Object
	imageinspected, err := imageManager.client.InspectImage(container.Image)
	if err != nil {
		seelog.Errorf("Error inspecting image: %v", err)
		return err
	}
	repository, tag := docker.ParseRepositoryTag(container.Image)
	var sourceImage *Image
	sourceImage = &Image{
		ImageId: imageinspected.ID,
		Size:    imageinspected.Size,
	}
	sourceImage.Names = append(sourceImage.Names, container.Image)
	sourceImage.Repos = append(sourceImage.Repos, repository)
	sourceImage.Tags = append(sourceImage.Tags, tag)
	sourceImageState := &ImageState{
		Image:      sourceImage,
		PulledTime: time.Now(),
	}
	seelog.Infof("Adding container reference- %v to Image state %v", container.Name, sourceImage.ImageId)
	sourceImageState.Containers = append(sourceImageState.Containers, container)
	imageManager.AddImageState(sourceImageState)
	return nil
}

// RemoveContainerReferenceFromImageState removes container reference from the corresponding imageState object
func (imageManager *dockerImageManager) RemoveContainerReferenceFromImageState(container *api.Container) error {
	if container.Image == "" {
		return fmt.Errorf("Invalid container reference: Empty image name")
	}
	// Find image state that this container is part of, and remove the reference
	imageState, ok := imageManager.getImageState(container)
	if !ok {
		return errors.New("Cannot find image state for the container to be removed")
	}
	// Found matching ImageState
	for i := len(imageState.Containers) - 1; i >= 0; i-- {
		if imageState.Containers[i].Name == container.Name {
			// Container reference found; hence remove it
			seelog.Infof("Removing Container Reference: %v from Image State- %v", container.Name, imageState.Image.ImageId)
			imageState.updateLock.Lock()
			defer imageState.updateLock.Unlock()
			imageState.Containers = append(imageState.Containers[:i], imageState.Containers[i+1:]...)
			// Update the last used time for the image
			imageState.LastUsedTime = time.Now()
			return nil
		}
	}
	return errors.New("Container reference is not found in the image state")
}

// getImageState returns the ImageState object that the container is referenced at
func (imageManager *dockerImageManager) getImageState(container *api.Container) (*ImageState, bool) {
	imageinspected, err := imageManager.client.InspectImage(container.Image)
	if err != nil {
		seelog.Errorf("Error inspecting image: %v", err)
		return nil, false
	}
	for _, imageState := range imageManager.getAllImageStates() {
		if imageState.Image.ImageId == imageinspected.ID {
			return imageState, true
		}
	}
	return nil, false
}

func (imageManager *dockerImageManager) hasImageName(imageState *ImageState, containerImageName string) bool {
	imageState.Image.imageLock.RLock()
	defer imageState.Image.imageLock.RUnlock()
	for _, imageName := range imageState.Image.Names {
		if imageName == containerImageName {
			return true
		}
	}
	return false
}

func (imageManager *dockerImageManager) GetEligibleImagesForDeletion() []*ImageState {
	if len(imageManager.imageStates) < 1 {
		return nil
	}
	var imagesForDeletion []*ImageState
	for _, imageState := range imageManager.getAllImageStates() {
		if imageManager.isImageOldEnough(imageState) && hasNoAssociatedContainers(imageState) {
			seelog.Infof("Image eligible for deletion: %+v", imageState)
			imagesForDeletion = append(imagesForDeletion, imageState)
		}
	}
	return imagesForDeletion
}

func hasNoAssociatedContainers(imageState *ImageState) bool {
	return len(imageState.Containers) == 0
}

func (imageManager *dockerImageManager) isImageOldEnough(imageState *ImageState) bool {
	ageOfImage := time.Now().Sub(imageState.PulledTime)
	return imageManager.minAge < ageOfImage
}

func (imageStates ImageStatesForDeletion) Len() int {
	return len(imageStates)
}

func (imageStates ImageStatesForDeletion) Less(i, j int) bool {
	return imageStates[i].LastUsedTime.Before(imageStates[j].LastUsedTime)
}

func (imageStates ImageStatesForDeletion) Swap(i, j int) {
	imageStates[i], imageStates[j] = imageStates[j], imageStates[i]
}

func (imageManager *dockerImageManager) GetLeastRecentlyUsedImages(imagesForDeletion ImageStatesForDeletion) ImageStatesForDeletion {
	if len(imagesForDeletion) <= IMAGES_TO_DELETE {
		return imagesForDeletion
	}
	// sort images in the order of last used times
	sort.Sort(imagesForDeletion)
	return imagesForDeletion[:IMAGES_TO_DELETE]
}
