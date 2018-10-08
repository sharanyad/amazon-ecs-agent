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

package factory

import (
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

// NVMLFactory wraps around the global functions exposed by NVML go bindings
type NVMLFactory interface {
	// Init is used to initialize NVML
	Init() error
	// Shutdown is used to shutdown NVML
	Shutdown() error
	// GetDeviceCount is used to get the number of GPUs in the instance
	GetDeviceCount() (uint, error)
	// NewDeviceLite is used to obtain the GPU device information
	NewDeviceLite(uint) (*nvml.Device, error)
}

// GlobalNVMLFactory calls the NVML go bindings global functions
type GlobalNVMLFactory struct{}

// Init is used to initialize NVML
func (n *GlobalNVMLFactory) Init() error {
	return nvml.Init()
}

// Shutdown is used to shutdown NVML
func (n *GlobalNVMLFactory) Shutdown() error {
	return nvml.Shutdown()
}

// GetDeviceCount is used to get the number of GPUs in the instance
func (n *GlobalNVMLFactory) GetDeviceCount() (uint, error) {
	return nvml.GetDeviceCount()
}

// GetDeviceCount is used to get the number of GPUs in the instance
func (n *GlobalNVMLFactory) NewDeviceLite(idx uint) (*nvml.Device, error) {
	return nvml.NewDeviceLite(idx)
}
