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

// The DockerTaskEngine is an abstraction over the DockerGoClient so that
// it does not have to know about tasks, only containers
package engine

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-agent/agent/api"
	"github.com/aws/amazon-ecs-agent/agent/statemanager"
	docker "github.com/fsouza/go-dockerclient"
	"golang.org/x/net/context"
)

// Deletion of images in the order of LRU time  : Happy path
//  a.       This includes starting up agent, pull images, start containers,
//   account them in image manager,  stop containers, remove containers, account this in image manager,
//  b.      Simulate the pulled time (so that it passes the minimum age criteria
//   for getting chosen for deletion )
//  c.       Start image cleanup , ensure that ONLY the top 2 eligible LRU images
//   are removed from the instance,  and those deleted images’ image states are removed from image manager.
//  d. Ensure images that do not pass the ‘minimumAgeForDeletion’ criteria are not removed.
//  e. Image has not passed the ‘hasNoAssociatedContainers’ criteria.
//  f. Ensure that that if not eligible, image is not deleted from the instance and image reference in ImageManager is not removed.
func TestIntegImageCleanupHappyCase(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.TaskCleanupWaitDuration = 5 * time.Second
	// start agent
	taskEngine, done, _ := setup(cfg, t)

	testImage1Name := "127.0.0.1:51670/amazon/test-image1:latest"
	testImage2Name := "127.0.0.1:51670/amazon/test-image2:latest"
	testImage3Name := "127.0.0.1:51670/amazon/test-image3:latest"

	// Create a context
	parent := context.Background()
	ctx, cancel := context.WithCancel(parent)

	imageManager := taskEngine.(*DockerTaskEngine).imageManager.(*dockerImageManager)

	defer func() {
		cancel()
		done()
		// Force cleanup all test images and containers
		imageManager.client.RemoveImageExtended(testImage1Name, docker.RemoveImageOptions{Force: true})
		imageManager.client.RemoveImageExtended(testImage2Name, docker.RemoveImageOptions{Force: true})
		imageManager.client.RemoveImageExtended(testImage3Name, docker.RemoveImageOptions{Force: true})
		imageManager.client.RemoveContainer("test1")
		imageManager.client.RemoveContainer("test2")
		imageManager.client.RemoveContainer("test3")
	}()

	// Set low values so this test can complete in a sane amout of time
	imageManager.minimumAgeBeforeDeletion = 5 * time.Second
	imageManager.numImagesToDelete = 2
	imageManager.imageCleanupTimeInterval = 60 * time.Second
	imageManager.SetSaver(statemanager.NewNoopStateManager())

	taskEvents, containerEvents := taskEngine.TaskEvents()

	defer discardEvents(containerEvents)()

	// Create test Task
	taskName := "imgClean"
	testTask := createImageCleanupTestTask(taskName)

	go taskEngine.AddTask(testTask)

	// Verify that Task is running
	err := verifyTaskIsRunning(taskEvents, testTask)
	if err != nil {
		t.Fatal(err)
	}

	// Start the image cleanup periodic task
	go imageManager.StartImageCleanupProcess(ctx)

	// Set the ImageState.LastUsedAt to a value far in the past to ensure the test images are deleted
	imageState1, ok := imageManager.getImageStateByName(testImage1Name)
	if !ok {
		t.Fatalf("Could not find image state for %s", testImage1Name)
	} else {
		t.Logf("Found image state for %s", testImage1Name)
	}
	imageState2, ok := imageManager.getImageStateByName(testImage2Name)
	if !ok {
		t.Fatalf("Could not find image state for %s", testImage2Name)
	} else {
		t.Logf("Found image state for %s", testImage2Name)
	}
	imageState3, ok := imageManager.getImageStateByName(testImage3Name)
	if !ok {
		t.Fatalf("Could not find image state for %s", testImage3Name)
	} else {
		t.Logf("Found image state for %s", testImage3Name)
	}

	imageState1ImageId := imageState1.Image.ImageId
	imageState2ImageId := imageState2.Image.ImageId
	imageState3ImageId := imageState3.Image.ImageId

	// This will make this the most LRU image
	imageState1.LastUsedAt = imageState1.LastUsedAt.Add(-99995 * time.Hour)
	imageState2.LastUsedAt = imageState2.LastUsedAt.Add(-99994 * time.Hour)
	imageState3.LastUsedAt = imageState3.LastUsedAt.Add(-99993 * time.Hour)

	// Verify Task is stopped.
	verifyTaskIsStopped(taskEvents, testTask)

	// Allow Task cleanup to occur
	time.Sleep(15 * time.Second)

	// Verify Task is cleaned up
	err = verifyTaskIsCleanedUp(taskName, taskEngine)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for image removal job completes
	time.Sleep(65 * time.Second)

	// Verify top 2 LRU images are deleted from image manager
	imageState1, ok = imageManager.getImageState(imageState1ImageId)
	if !ok {
		t.Logf("Could not find %s. Has been removed from ImageManager", imageState1ImageId)
	} else {
		t.Fatalf("ImageID %s was not removed. Name: %s", imageState1ImageId, strings.Join(imageState1.Image.Names, " "))
	}
	imageState2, ok = imageManager.getImageState(imageState2ImageId)
	if !ok {
		t.Logf("Could not find %s. Has been removed from ImageManager", imageState2ImageId)
	} else {
		t.Fatalf("ImageID %s was not removed. Name: %s", imageState2ImageId, strings.Join(imageState2.Image.Names, " "))
	}

	// Verify 3rd LRU image is not removed
	imageState3, ok = imageManager.getImageState(imageState3ImageId)
	if ok {
		t.Logf("ImageID %s was not removed. Name: %s", imageState3ImageId, strings.Join(imageState3.Image.Names, " "))
	} else {
		t.Fatalf("Could not find %s. Has been removed from ImageManager", imageState3ImageId)
	}

	// Verify top 2 LRU images are removed from docker
	_, err = taskEngine.(*DockerTaskEngine).client.InspectImage(imageState1ImageId)
	if err != docker.ErrNoSuchImage {
		t.Fatalf("Image was not removed successfully")
	}
	_, err = taskEngine.(*DockerTaskEngine).client.InspectImage(imageState2ImageId)
	if err != docker.ErrNoSuchImage {
		t.Fatalf("Image was not removed successfully")
	}

	// Verify 3rd LRU image has not been removed from Docker
	_, err = taskEngine.(*DockerTaskEngine).client.InspectImage(imageState3ImageId)
	if err != nil {
		t.Fatalf("Image should not have been removed from Docker")
	}
}

// Non deletion of images not falling in the eligibility criteria for getting
//  chosen for deletion :
//  a. Ensure images that do not pass the ‘minimumAgeForDeletion’ criteria are not removed.
//  b. Image has not passed the ‘hasNoAssociatedContainers’ criteria.
//  c. Ensure that the image is not deleted from the instance and image reference in ImageManager is not removed.
func TestIntegImageCleanupThreshold(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.TaskCleanupWaitDuration = 5 * time.Second
	// start agent
	taskEngine, done, _ := setup(cfg, t)

	testImage1Name := "127.0.0.1:51670/amazon/test-image1:latest"
	testImage2Name := "127.0.0.1:51670/amazon/test-image2:latest"
	testImage3Name := "127.0.0.1:51670/amazon/test-image3:latest"

	// Create a context
	parent := context.Background()
	ctx, cancel := context.WithCancel(parent)

	imageManager := taskEngine.(*DockerTaskEngine).imageManager.(*dockerImageManager)

	defer func() {
		cancel()
		done()
		// Force cleanup all test images and containers
		imageManager.client.RemoveImageExtended(testImage1Name, docker.RemoveImageOptions{Force: true})
		imageManager.client.RemoveImageExtended(testImage2Name, docker.RemoveImageOptions{Force: true})
		imageManager.client.RemoveImageExtended(testImage3Name, docker.RemoveImageOptions{Force: true})
		imageManager.client.RemoveContainer("test1")
		imageManager.client.RemoveContainer("test2")
		imageManager.client.RemoveContainer("test3")
	}()

	// Set low values so this test can complete in a sane amout of time
	imageManager.imageCleanupTimeInterval = 60 * time.Second
	imageManager.minimumAgeBeforeDeletion = 15 * time.Minute

	// Set to delete three images, but in this test we expect only two images to be removed
	imageManager.numImagesToDelete = 3
	imageManager.SetSaver(statemanager.NewNoopStateManager())

	taskEvents, containerEvents := taskEngine.TaskEvents()
	defer discardEvents(containerEvents)()

	// Create test Task
	taskName := "imgClean"
	testTask := createImageCleanupTestTask(taskName)

	// Start Task
	go taskEngine.AddTask(testTask)

	// Verify that Task is running
	err := verifyTaskIsRunning(taskEvents, testTask)
	if err != nil {
		t.Fatal(err)
	}

	// Start the image cleanup periodic task
	go imageManager.StartImageCleanupProcess(ctx)

	// Set the ImageState.LastUsedAt to a value far in the past to ensure the test images are deleted
	imageState1, ok := imageManager.getImageStateByName(testImage1Name)
	if !ok {
		t.Fatalf("Could not find image state for %s", testImage1Name)
	} else {
		t.Logf("Found image state for %s", testImage1Name)
	}
	imageState2, ok := imageManager.getImageStateByName(testImage2Name)
	if !ok {
		t.Fatalf("Could not find image state for %s", testImage2Name)
	} else {
		t.Logf("Found image state for %s", testImage2Name)
	}
	imageState3, ok := imageManager.getImageStateByName(testImage3Name)
	if !ok {
		t.Fatalf("Could not find image state for %s", testImage3Name)
	} else {
		t.Logf("Found image state for %s", testImage3Name)
	}

	imageState1ImageId := imageState1.Image.ImageId
	imageState2ImageId := imageState2.Image.ImageId
	imageState3ImageId := imageState3.Image.ImageId

	// This will make these the most LRU images so they are deleted
	imageState1.LastUsedAt = imageState1.LastUsedAt.Add(-99995 * time.Hour)
	imageState2.LastUsedAt = imageState2.LastUsedAt.Add(-99994 * time.Hour)
	imageState3.LastUsedAt = imageState3.LastUsedAt.Add(-99993 * time.Hour)

	// Set two containers to have pull time > threshold
	imageState1.PulledAt = imageState1.PulledAt.Add(-20 * time.Minute)
	imageState2.PulledAt = imageState2.PulledAt.Add(-10 * time.Minute)
	imageState3.PulledAt = imageState3.PulledAt.Add(-25 * time.Minute)

	// Verify Task is stopped
	verifyTaskIsStopped(taskEvents, testTask)

	// Allow Task cleanup to occur
	time.Sleep(15 * time.Second)

	// Verify Task is cleaned up
	err = verifyTaskIsCleanedUp(taskName, taskEngine)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for image removal job completes
	time.Sleep(65 * time.Second)

	// Verify Image1 & Image3 are removed from ImageManager as they are beyond the minimumAge threshold
	imageState1, ok = imageManager.getImageState(imageState1ImageId)
	if !ok {
		t.Logf("Could not find %s. Has been removed from ImageManager", imageState1ImageId)
	} else {
		t.Fatalf("ImageID %s was not removed. Name: %s", imageState1ImageId, strings.Join(imageState1.Image.Names, " "))
	}
	imageState3, ok = imageManager.getImageState(imageState3ImageId)
	if !ok {
		t.Logf("Could not find %s. Has been removed from ImageManager", imageState3ImageId)
	} else {
		t.Fatalf("ImageID %s was not removed. Name: %s", imageState3ImageId, strings.Join(imageState3.Image.Names, " "))
	}

	// Verify Image2 is not removed, below threshold for minimumAge
	imageState2, ok = imageManager.getImageState(imageState2ImageId)
	if ok {
		t.Logf("ImageID %s was not removed. Name: %s", imageState2ImageId, strings.Join(imageState2.Image.Names, " "))
	} else {
		t.Fatalf("Could not find %s. Has been removed from ImageManager", imageState2ImageId)
	}

	// Verify Image1 & Image3 are removed from docker
	_, err = taskEngine.(*DockerTaskEngine).client.InspectImage(imageState1ImageId)
	if err != docker.ErrNoSuchImage {
		t.Fatalf("Image was not removed successfully")
	}
	_, err = taskEngine.(*DockerTaskEngine).client.InspectImage(imageState3ImageId)
	if err != docker.ErrNoSuchImage {
		t.Fatalf("Image was not removed successfully")
	}

	// Verify Image2 has not been removed from Docker
	_, err = taskEngine.(*DockerTaskEngine).client.InspectImage(imageState2ImageId)
	if err != nil {
		t.Fatalf("Image should not have been removed from Docker")
	}
}

// Test Same image, two conteiners, hasNoAssociatedContainers
func TestIntegSameImageTwoContainers(t *testing.T) {

}

// Choose an image for deletion; try pulling a container at the same time
//  Based on the same image chosen for deletion; ensure that pull happens only
//  after the deletion of the image is complete.
func TestIntegImageCleanupConcurrentPullAndDelete(t *testing.T) {

}

// Delete larger images in a row. Ensure that it doesn’t timeout and that it
//  deletes all the images
func TestIntegImageCleanupLargeImages(t *testing.T) {

}

// Deleting an invalid image returns an error but does not cause other failures
//  e.g. Image is in imageManager but not on System.
// 	Ensure image state is removed for that image
func TestIntegImageCleanupImageDeleteFails(t *testing.T) {

}

func verifyTaskIsRunning(taskEvents <-chan api.TaskStateChange, testTask *api.Task) error {
	for {
		select {
		case taskEvent := <-taskEvents:
			if taskEvent.TaskArn != testTask.Arn {
				continue
			}
			if taskEvent.Status == api.TaskRunning {
				return nil
			} else if taskEvent.Status > api.TaskRunning {
				return errors.New("Task went straight to " + taskEvent.Status.String() + " without running")
			}
		}
	}
}

func verifyTaskIsStopped(taskEvents <-chan api.TaskStateChange, testTask *api.Task) {
	for {
		select {
		case taskEvent := <-taskEvents:
			if taskEvent.TaskArn != testTask.Arn {
				continue
			}
			if taskEvent.Status >= api.TaskStopped {
				return
			}
		}
	}
}

func verifyTaskIsCleanedUp(taskName string, taskEngine TaskEngine) error {
	for i := 0; i < 70; i++ {
		_, ok := taskEngine.(*DockerTaskEngine).State().TaskByArn(taskName)
		if !ok {
			break
		}
		time.Sleep(1 * time.Second)
		if i == 69 {
			return errors.New("Expected Task to have been swept but was not")
		}
	}
	return nil
}
