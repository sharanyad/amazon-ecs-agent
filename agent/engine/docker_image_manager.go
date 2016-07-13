package engine

import (
	"sync"
	"time"

	"github.com/aws/amazon-ecs-agent/agent/api"
	"github.com/cihub/seelog"
	docker "github.com/fsouza/go-dockerclient"
)

// ImageManager is responsible for saving the Image states,
// adding and removing container references to ImageStates
type ImageManager interface {
	AddImageState(imageState *ImageState)
	AddContainerReferenceToImageState(container *api.Container) error
	RemoveContainerReferenceFromImageState(container *api.Container)
}

// Image is the type representing a docker image and its various properties
type Image struct {
	ImageId string
	Name    string
	Repo    string
	Tag     string
	Size    int64
}

// ImageState is the type representing a docker image
// and its state information such as containers associated with it
type ImageState struct {
	Image        *Image
	Containers   []*api.Container
	PulledTime   time.Time
	LastUsedTime time.Time
}

// DockerImageManager accounts all the images and their states in the instance.
// It also has the cleanup policy configuration.
type dockerImageManager struct {
	imageStates []*ImageState
	// TODO: add cleanup policy details
	imageStateLock sync.RWMutex
	client         DockerClient
}

func NewImageManager(client DockerClient) ImageManager {
	return &dockerImageManager{
		client: client,
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
	imageState, exist := imageManager.getImageState(container)
	if exist {
		// Image State already exists; add Container to it
		seelog.Infof("Adding container reference- %v to Image state- %v", container.Name, imageState.Image.Name)
		imageState.Containers = append(imageState.Containers, container)
		return nil
	}
	// Inspect image for creating new Image Object
	imageinspected, err := imageManager.client.InspectImage(container.Image)
	if err != nil {
		seelog.Debugf("Bad image: %v", container.Image)
		return err
	}
	repository, tag := docker.ParseRepositoryTag(container.Image)
	var sourceImage *Image
	sourceImage = &Image{
		ImageId: imageinspected.ID,
		Name:    container.Image,
		Repo:    repository,
		Tag:     tag,
		Size:    imageinspected.Size,
	}
	sourceImageState := &ImageState{
		Image:      sourceImage,
		PulledTime: time.Now(),
	}
	seelog.Infof("Adding container reference- %v to Image state %v", container.Name, sourceImage.Name)
	sourceImageState.Containers = append(sourceImageState.Containers, container)
	imageManager.AddImageState(sourceImageState)
	return nil
}

// RemoveContainerReferenceFromImageState removes container reference from the corresponding imageState object
func (imageManager *dockerImageManager) RemoveContainerReferenceFromImageState(container *api.Container) {
	// Find image states that this container is part of, and remove the reference
	imageState, ok := imageManager.getImageState(container)
	if ok {
		// Found matching ImageState
		for i := len(imageState.Containers) - 1; i >= 0; i-- {
			if imageState.Containers[i].Name == container.Name {
				// Container reference found; hence remove it
				seelog.Infof("Removing Container Reference: %v from Image State- %v", container.Name, imageState.Image.Name)
				imageState.Containers = append(imageState.Containers[:i], imageState.Containers[i+1:]...)
				// Update the last used time for the image
				imageState.LastUsedTime = time.Now()
			}
		}
	}
}

// getImageState returns the ImageState object that the container is referenced at
func (imageManager *dockerImageManager) getImageState(container *api.Container) (*ImageState, bool) {
	for _, imageState := range imageManager.getAllImageStates() {
		if imageState.Image.Name == container.Image {
			return imageState, true
		}
	}
	return nil, false
}
