// +build !linux
// Copyright 2014-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package cgroup

import (
	"sync"
	"time"

	"github.com/aws/amazon-ecs-agent/agent/taskresource"
)

const (
	resourceName = "cgroup"
)

// CgroupResource represents Cgroup resource
type CgroupResource struct {
	desiredStatusUnsafe taskresource.ResourceStatus
	knownStatusUnsafe   taskresource.ResourceStatus
	appliedStatus       taskresource.ResourceStatus
	// lock is used for fields that are accessed and updated concurrently
	lock sync.RWMutex
}

// SetDesiredStatus safely sets the desired status of the resource
func (c *CgroupResource) SetDesiredStatus(status taskresource.ResourceStatus) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.desiredStatusUnsafe = status
}

// GetDesiredStatus safely returns the desired status of the task
func (c *CgroupResource) GetDesiredStatus() taskresource.ResourceStatus {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.desiredStatusUnsafe
}

// GetName safely returns the name of the resource
func (c *CgroupResource) GetName() string {
	return resourceName
}

// DesiredTerminal returns true if the cgroup's desired status is REMOVED
func (c *CgroupResource) DesiredTerminal() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.desiredStatusUnsafe == taskresource.ResourceRemoved
}

// KnownCreated returns true if the cgroup's known status is CREATED
func (c *CgroupResource) KnownCreated() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.knownStatusUnsafe == taskresource.ResourceCreated
}

// TerminalStatus returns the last transition state of cgroup
func (c *CgroupResource) TerminalStatus() taskresource.ResourceStatus {
	return taskresource.ResourceRemoved
}

// GetNextKnownStateProgression returns the state that the resource should
// progress to based on its `KnownState`.
func (c *CgroupResource) GetNextKnownStateProgression() taskresource.ResourceStatus {
	return c.GetKnownStatus() + 1
}

// ApplyTransition calls the function required to move to the specified status
func (c *CgroupResource) ApplyTransition(nextState taskresource.ResourceStatus) (bool, error) {
	return true, nil
}

// SteadyState returns the transition state of the resource defined as "ready"
func (c *CgroupResource) SteadyState() taskresource.ResourceStatus {
	return taskresource.ResourceCreated
}

// SetKnownStatus safely sets the currently known status of the resource
func (c *CgroupResource) SetKnownStatus(status taskresource.ResourceStatus) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.knownStatusUnsafe = status
	c.updateAppliedStatusUnsafe(status)
}

// updateAppliedStatusUnsafe updates the resource transitioning status
func (c *CgroupResource) updateAppliedStatusUnsafe(knownStatus taskresource.ResourceStatus) {}

// SetAppliedStatus sets the applied status of resource and returns whether
// the resource is already in a transition
func (c *CgroupResource) SetAppliedStatus(status taskresource.ResourceStatus) bool {
	return true
}

// GetKnownStatus safely returns the currently known status of the task
func (c *CgroupResource) GetKnownStatus() taskresource.ResourceStatus {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.knownStatusUnsafe
}

// SetCreatedAt sets the timestamp for resource's creation time
func (c *CgroupResource) SetCreatedAt(createdAt time.Time) {}

// GetCreatedAt sets the timestamp for resource's creation time
func (c *CgroupResource) GetCreatedAt() time.Time {
	return time.Time{}
}

func (c *CgroupResource) setupTaskCgroup() error {
	return nil
}

// Create creates cgroup root for the task
func (c *CgroupResource) Create() error {
	return nil
}

// Cleanup removes the cgroup root created for the task
func (c *CgroupResource) Cleanup() error {
	return nil
}

// StatusString returns the string of the cgroup resource status
func (c *CgroupResource) StatusString(status taskresource.ResourceStatus) string {
	return CgroupStatus(status).String()
}

// cgroupResourceJSON duplicates CgroupResource fields, only for marshalling and unmarshalling purposes
type cgroupResourceJSON struct{}

// MarshalJSON marshals CgroupResource object using duplicate struct CgroupResourceJSON
func (c *CgroupResource) MarshalJSON() ([]byte, error) {
	return nil, nil
}

// UnmarshalJSON unmarshals CgroupResource object using duplicate struct CgroupResourceJSON
func (c *CgroupResource) UnmarshalJSON(b []byte) error {
	return nil
}
