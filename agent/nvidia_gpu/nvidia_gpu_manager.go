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
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/cihub/seelog"

	"github.com/pkg/errors"
)

// NvidiaGPUManager is used as a wrapper for NVML APIs
type NvidiaGPUManager struct{}

// NewNvidiaGPUManager is used to obtain a new NvidiaGPUManager object
func NewNvidiaGPUManager() *NvidiaGPUManager {
	return &NvidiaGPUManager{}
}

// Initialize is for initlializing nvidia's nvml library
func (n *NvidiaGPUManager) Initialize() error {
	err := InitNVML()
	if err != nil {
		return errors.Wrapf(err, "nvidia gpu manager: error initializing nvidia nvml")
	}
	return nil
}

var InitNVML = Init

func Init() error {
	return nvml.Init()
}

// Shutdown is for shutting down nvidia's nvml library
func (n *NvidiaGPUManager) Shutdown() error {
	err := ShutdownNVML()
	if err != nil {
		return errors.Wrapf(err, "nvidia gpu manager: error shutting down nvidia nvml")
	}
	return nil
}

var ShutdownNVML = Shutdown

func Shutdown() error {
	return nvml.Shutdown()
}

var NvmlGetDeviceCount = GetDeviceCount

func GetDeviceCount() (uint, error) {
	return nvml.GetDeviceCount()
}

var NvmlNewDeviceLite = NewDeviceLite

func NewDeviceLite(idx uint) (*nvml.Device, error) {
	return nvml.NewDeviceLite(idx)
}

// GetGPUDeviceIDs is for getting the GPU device UUIDs
func (n *NvidiaGPUManager) GetGPUDeviceIDs() ([]string, error) {
	count, err := NvmlGetDeviceCount()
	if err != nil {
		return nil, errors.Wrapf(err, "nvidia gpu manager: error getting GPU device count for UUID detection")
	}
	var gpuIds []string
	var i uint
	for i = 0; i < count; i++ {
		device, err := NvmlNewDeviceLite(i)
		if err != nil {
			seelog.Errorf("nvidia gpu manager: error initializing device of index %d: %v", i, err)
			continue
		}
		gpuIds = append(gpuIds, device.UUID)
	}
	if len(gpuIds) == 0 {
		return gpuIds, errors.New("nvidia gpu manager: error initializing GPU devices")
	}
	return gpuIds, nil
}

// GetGPUDeviceModels is for getting the GPU device models
func (n *NvidiaGPUManager) GetGPUDeviceModels() ([]string, error) {
	count, err := NvmlGetDeviceCount()
	if err != nil {
		return nil, errors.Wrapf(err, "nvidia gpu manager: error getting GPU device count for model detection")
	}
	var gpuModels []string
	var i uint
	for i = 0; i < count; i++ {
		device, err := NvmlNewDeviceLite(i)
		if err != nil {
			seelog.Errorf("nvidia gpu manager: error initializing device of index %d", i)
			continue
		}
		gpuModels = append(gpuModels, *device.Model)
	}
	if len(gpuModels) == 0 {
		return gpuModels, errors.New("nvidia gpu manager: error initializing GPU devices")
	}
	return gpuModels, nil
}
