// Copyright 2015-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/aws/amazon-ecs-agent/agent/containermetadata (interfaces: Manager,DockerMetadataClient)

// Package mock_containermetadata is a generated GoMock package.
package mock_containermetadata

import (
	context "context"
	reflect "reflect"
	time "time"

	task "github.com/aws/amazon-ecs-agent/agent/api/task"
	types "github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	gomock "github.com/golang/mock/gomock"
)

// MockManager is a mock of Manager interface
type MockManager struct {
	ctrl     *gomock.Controller
	recorder *MockManagerMockRecorder
}

// MockManagerMockRecorder is the mock recorder for MockManager
type MockManagerMockRecorder struct {
	mock *MockManager
}

// NewMockManager creates a new mock instance
func NewMockManager(ctrl *gomock.Controller) *MockManager {
	mock := &MockManager{ctrl: ctrl}
	mock.recorder = &MockManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockManager) EXPECT() *MockManagerMockRecorder {
	return m.recorder
}

// Clean mocks base method
func (m *MockManager) Clean(arg0 string) error {
	ret := m.ctrl.Call(m, "Clean", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Clean indicates an expected call of Clean
func (mr *MockManagerMockRecorder) Clean(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Clean", reflect.TypeOf((*MockManager)(nil).Clean), arg0)
}

// Create mocks base method
func (m *MockManager) Create(arg0 *container.Config, arg1 *container.HostConfig, arg2 *task.Task, arg3 string) error {
	ret := m.ctrl.Call(m, "Create", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create
func (mr *MockManagerMockRecorder) Create(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockManager)(nil).Create), arg0, arg1, arg2, arg3)
}

// SetContainerInstanceARN mocks base method
func (m *MockManager) SetContainerInstanceARN(arg0 string) {
	m.ctrl.Call(m, "SetContainerInstanceARN", arg0)
}

// SetContainerInstanceARN indicates an expected call of SetContainerInstanceARN
func (mr *MockManagerMockRecorder) SetContainerInstanceARN(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetContainerInstanceARN", reflect.TypeOf((*MockManager)(nil).SetContainerInstanceARN), arg0)
}

// Update mocks base method
func (m *MockManager) Update(arg0 context.Context, arg1 string, arg2 *task.Task, arg3 string) error {
	ret := m.ctrl.Call(m, "Update", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update
func (mr *MockManagerMockRecorder) Update(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockManager)(nil).Update), arg0, arg1, arg2, arg3)
}

// MockDockerMetadataClient is a mock of DockerMetadataClient interface
type MockDockerMetadataClient struct {
	ctrl     *gomock.Controller
	recorder *MockDockerMetadataClientMockRecorder
}

// MockDockerMetadataClientMockRecorder is the mock recorder for MockDockerMetadataClient
type MockDockerMetadataClientMockRecorder struct {
	mock *MockDockerMetadataClient
}

// NewMockDockerMetadataClient creates a new mock instance
func NewMockDockerMetadataClient(ctrl *gomock.Controller) *MockDockerMetadataClient {
	mock := &MockDockerMetadataClient{ctrl: ctrl}
	mock.recorder = &MockDockerMetadataClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockDockerMetadataClient) EXPECT() *MockDockerMetadataClientMockRecorder {
	return m.recorder
}

// InspectContainer mocks base method
func (m *MockDockerMetadataClient) InspectContainer(arg0 context.Context, arg1 string, arg2 time.Duration) (*types.ContainerJSON, error) {
	ret := m.ctrl.Call(m, "InspectContainer", arg0, arg1, arg2)
	ret0, _ := ret[0].(*types.ContainerJSON)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InspectContainer indicates an expected call of InspectContainer
func (mr *MockDockerMetadataClientMockRecorder) InspectContainer(arg0, arg1, arg2 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InspectContainer", reflect.TypeOf((*MockDockerMetadataClient)(nil).InspectContainer), arg0, arg1, arg2)
}
