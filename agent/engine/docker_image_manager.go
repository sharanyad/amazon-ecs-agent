package engine

import (
	"sync"
	"time"

	"github.com/aws/amazon-ecs-agent/agent/api"
	"github.com/cihub/seelog"
	docker "github.com/fsouza/go-dockerclient"
)

type ImageManager interface {
	addImageState(imageState *ImageState)
	addContainerToImage(container *api.Container) error
	removeContainerFromImage(container *api.Container)
}

type Image struct {
	ImageId string
	Name    string
	Repo    string
	Tag     string
	Size    int64
}

type ImageState struct {
	Image        *Image
	Containers   []*api.Container
	PulledTime   time.Time
	lastUsedTime time.Time
}

// ImageManager accounts all the images and their states in the instance.
// It also has the cleanup policy configuration.
type DockerImageManager struct {
	ImageStates []*ImageState
	// TODO: add cleanup policy details
	ImageStateLock sync.Mutex
}

func NewImageManager() ImageManager {
	imageManager := &DockerImageManager{}
	return imageManager
}

func (imageManager *DockerImageManager) addImageState(imageState *ImageState) {
	imageManager.ImageStateLock.Lock()
	defer imageManager.ImageStateLock.Unlock()
	imageManager.ImageStates = append(imageManager.ImageStates, imageState)
}

func (imageManager *DockerImageManager) getImageStates() []*ImageState {
	imageManager.ImageStateLock.Lock()
	defer imageManager.ImageStateLock.Unlock()
	return imageManager.ImageStates
}

func (imageManager *DockerImageManager) addContainerToImage(container *api.Container) error {
	for _, imageState := range imageManager.getImageStates() {
		if imageState.Image.Name == container.Image {
			// Image State already exists; add Container to it
			seelog.Infof("Adding container reference- %v to Image state- %v", container.Name, imageState.Image.Name)
			imageState.Containers = append(imageState.Containers, container)
			return nil
		}
	}
	// Inspect image for creating new Image Object
	dockerClient, err := docker.NewClientFromEnv()
	if err != nil {
		log.Debug("cannot create Docker Client")
		return err
	}
	imageinspected, error := dockerClient.InspectImage(container.Image)
	if error != nil {
		log.Debug("Bad image", "image", container.Image)
		return error
	} else {
		repository, tag := docker.ParseRepositoryTag(container.Image)
		var sourceImage *Image
		sourceImage = &Image{
			ImageId: imageinspected.ID,
			Name:    container.Image,
			Repo:    repository,
			Tag:     tag,
			Size:    imageinspected.Size,
		}
		imageState := &ImageState{
			Image:      sourceImage,
			PulledTime: time.Now(),
		}
		seelog.Infof("Adding container reference- %v to Image state %v", container.Name, sourceImage.Name)
		imageState.Containers = append(imageState.Containers, container)
		imageManager.addImageState(imageState)
		return nil
	}
}

func (imageManager *DockerImageManager) removeContainerFromImage(container *api.Container) {
	// Find image states that this container is part of, and remove the reference
	for _, imageState := range imageManager.getImageStates() {
		if imageState.Image.Name == container.Image {
			// Found matching ImageState
			for i := len(imageState.Containers) - 1; i >= 0; i-- {
				if imageState.Containers[i].Name == container.Name {
					// Container reference found; hence remove it
					seelog.Infof("Removing Container Reference: %v from Image State- %v", container.Name, imageState.Image.Name)
					imageState.Containers = append(imageState.Containers[:i], imageState.Containers[i+1:]...)
					// Update the last used time for the image
					imageState.lastUsedTime = time.Now()
				}
			}
		}
	}
}
