// +build linux

// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package nvidia_gpu

import (
	"github.com/aws/amazon-ecs-agent/agent/nvidia_gpu/factory"
	"github.com/cihub/seelog"

	"github.com/pkg/errors"
)

type NvidiaGPUManager interface {
	Initialize() error
	Shutdown() error
	GetGPUDeviceIDs() ([]string, error)
	GetGPUDeviceModels() ([]string, error)
}

// nvidiaGPUManager is used as a wrapper for GlobalNVMLFactory and implements
// NvidiaGPUManager
type nvidiaGPUManager struct {
	factory.NVMLFactory
}

// New is used to obtain a new NvidiaGPUManager object
func NewNvidiaGPUManager() NvidiaGPUManager {
	return &nvidiaGPUManager{
		&factory.GlobalNVMLFactory{},
	}
}

// Initialize is for initlializing nvidia's nvml library
func (n *nvidiaGPUManager) Initialize() error {
	err := n.Init()
	if err != nil {
		return errors.Wrapf(err, "nvidia gpu manager: error initializing nvidia nvml")
	}
	return nil
}

// Shutdown is for shutting down nvidia's nvml library
func (n *nvidiaGPUManager) Shutdown() error {
	err := n.Shutdown()
	if err != nil {
		return errors.Wrapf(err, "nvidia gpu manager: error shutting down nvidia nvml")
	}
	return nil
}

// GetGPUDeviceIDs is for getting the GPU device UUIDs
func (n *nvidiaGPUManager) GetGPUDeviceIDs() ([]string, error) {
	count, err := n.GetDeviceCount()
	if err != nil {
		return nil, errors.Wrapf(err, "nvidia gpu manager: error getting GPU device count for UUID detection")
	}
	var gpuIds []string
	var i uint
	for i = 0; i < count; i++ {
		device, err := n.NewDeviceLite(i)
		if err != nil {
			seelog.Errorf("nvidia gpu manager: error initializing device of index %d", count)
			continue
		}
		gpuIds = append(gpuIds, device.UUID)
	}
	return gpuIds, nil
}

// GetGPUDeviceModels is for getting the GPU device models
func (n *nvidiaGPUManager) GetGPUDeviceModels() ([]string, error) {
	count, err := n.GetDeviceCount()
	if err != nil {
		return nil, errors.Wrapf(err, "nvidia gpu manager: error getting GPU device count for model detection")
	}
	var gpuModels []string
	var i uint
	for i = 0; i < count; i++ {
		device, err := n.NewDeviceLite(i)
		if err != nil {
			seelog.Errorf("nvidia gpu manager: error initializing device of index %d", count)
			continue
		}
		gpuModels = append(gpuModels, *device.Model)
	}
	return gpuModels, nil
}
