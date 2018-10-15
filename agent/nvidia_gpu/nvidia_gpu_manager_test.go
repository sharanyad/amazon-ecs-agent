// +build linux,unit

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
	"errors"
	"reflect"
	"testing"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/stretchr/testify/assert"
)

func TestNVMLInitialize(t *testing.T) {
	nvidiaGPUManager := NewNvidiaGPUManager()
	InitNVML = func() error {
		return nil
	}
	defer func() {
		InitNVML = Init
	}()
	err := nvidiaGPUManager.Initialize()
	assert.NoError(t, err)
}

func TestNVMLInitializeError(t *testing.T) {
	nvidiaGPUManager := NewNvidiaGPUManager()
	InitNVML = func() error {
		return errors.New("error initializing nvml")
	}
	defer func() {
		InitNVML = Init
	}()
	err := nvidiaGPUManager.Initialize()
	assert.Error(t, err)
}

func TestDeviceCount(t *testing.T) {
	NvmlGetDeviceCount = func() (uint, error) {
		return 1, nil
	}
	defer func() {
		NvmlGetDeviceCount = GetDeviceCount
	}()
	count, err := NvmlGetDeviceCount()
	assert.Equal(t, uint(1), count)
	assert.NoError(t, err)
}

func TestDeviceCountError(t *testing.T) {
	NvmlGetDeviceCount = func() (uint, error) {
		return 0, errors.New("device count error")
	}
	defer func() {
		NvmlGetDeviceCount = GetDeviceCount
	}()
	_, err := NvmlGetDeviceCount()
	assert.Error(t, err)
}

func TestNewDeviceLite(t *testing.T) {
	model := "Tesla-k80"
	NvmlNewDeviceLite = func(idx uint) (*nvml.Device, error) {
		return &nvml.Device{
			UUID:  "gpu-0123",
			Model: &model,
		}, nil
	}
	defer func() {
		NvmlNewDeviceLite = NewDeviceLite
	}()
	device, err := NvmlNewDeviceLite(4)
	assert.NoError(t, err)
	assert.Equal(t, "gpu-0123", device.UUID)
	assert.Equal(t, model, *device.Model)
}

func TestNewDeviceLiteError(t *testing.T) {
	NvmlNewDeviceLite = func(idx uint) (*nvml.Device, error) {
		return nil, errors.New("device error")
	}
	defer func() {
		NvmlNewDeviceLite = NewDeviceLite
	}()
	device, err := NvmlNewDeviceLite(4)
	assert.Error(t, err)
	assert.Nil(t, device)
}

func TestGetGPUDeviceIDs(t *testing.T) {
	nvidiaGPUManager := NewNvidiaGPUManager()
	NvmlGetDeviceCount = func() (uint, error) {
		return 1, nil
	}
	model := "Tesla-k80"
	NvmlNewDeviceLite = func(idx uint) (*nvml.Device, error) {
		return &nvml.Device{
			UUID:  "gpu-0123",
			Model: &model,
		}, nil
	}
	defer func() {
		NvmlGetDeviceCount = GetDeviceCount
		NvmlNewDeviceLite = NewDeviceLite
	}()
	gpuIds, err := nvidiaGPUManager.GetGPUDeviceIDs()
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual([]string{"gpu-0123"}, gpuIds))
}

func TestGetGPUDeviceIDsCountError(t *testing.T) {
	nvidiaGPUManager := NewNvidiaGPUManager()
	NvmlGetDeviceCount = func() (uint, error) {
		return 0, errors.New("device count error")
	}
	defer func() {
		NvmlGetDeviceCount = GetDeviceCount
	}()
	gpuIds, err := nvidiaGPUManager.GetGPUDeviceIDs()
	assert.Error(t, err)
	assert.Empty(t, gpuIds)
}

func TestGetGPUDeviceIDsDeviceError(t *testing.T) {
	nvidiaGPUManager := NewNvidiaGPUManager()
	NvmlGetDeviceCount = func() (uint, error) {
		return 1, nil
	}
	NvmlNewDeviceLite = func(idx uint) (*nvml.Device, error) {
		return nil, errors.New("device error")
	}
	defer func() {
		NvmlGetDeviceCount = GetDeviceCount
		NvmlNewDeviceLite = NewDeviceLite
	}()
	gpuIds, err := nvidiaGPUManager.GetGPUDeviceIDs()
	assert.Error(t, err)
	assert.Empty(t, gpuIds)
}

func TestGetGPUDeviceModels(t *testing.T) {
	nvidiaGPUManager := NewNvidiaGPUManager()
	NvmlGetDeviceCount = func() (uint, error) {
		return 1, nil
	}
	model := "Tesla-k80"
	NvmlNewDeviceLite = func(idx uint) (*nvml.Device, error) {
		return &nvml.Device{
			UUID:  "gpu-0123",
			Model: &model,
		}, nil
	}
	defer func() {
		NvmlGetDeviceCount = GetDeviceCount
		NvmlNewDeviceLite = NewDeviceLite
	}()
	gpuModels, err := nvidiaGPUManager.GetGPUDeviceModels()
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual([]string{"Tesla-k80"}, gpuModels))
}

func TestGetGPUDeviceModelsCountError(t *testing.T) {
	nvidiaGPUManager := NewNvidiaGPUManager()
	NvmlGetDeviceCount = func() (uint, error) {
		return 0, errors.New("device count error")
	}
	defer func() {
		NvmlGetDeviceCount = GetDeviceCount
	}()
	gpuModels, err := nvidiaGPUManager.GetGPUDeviceModels()
	assert.Error(t, err)
	assert.Empty(t, gpuModels)
}

func TestGetGPUDeviceModelsDeviceError(t *testing.T) {
	nvidiaGPUManager := NewNvidiaGPUManager()
	NvmlGetDeviceCount = func() (uint, error) {
		return 1, nil
	}
	NvmlNewDeviceLite = func(idx uint) (*nvml.Device, error) {
		return nil, errors.New("device error")
	}
	defer func() {
		NvmlGetDeviceCount = GetDeviceCount
		NvmlNewDeviceLite = NewDeviceLite
	}()
	gpuModels, err := nvidiaGPUManager.GetGPUDeviceModels()
	assert.Error(t, err)
	assert.Empty(t, gpuModels)
}

func TestNVMLShutdown(t *testing.T) {
	nvidiaGPUManager := NewNvidiaGPUManager()
	ShutdownNVML = func() error {
		return nil
	}
	defer func() {
		ShutdownNVML = Shutdown
	}()
	err := nvidiaGPUManager.Shutdown()
	assert.NoError(t, err)
}

func TestNVMLShutdownError(t *testing.T) {
	nvidiaGPUManager := NewNvidiaGPUManager()
	ShutdownNVML = func() error {
		return errors.New("error shutting down nvml")
	}
	defer func() {
		ShutdownNVML = Shutdown
	}()
	err := nvidiaGPUManager.Shutdown()
	assert.Error(t, err)
}
