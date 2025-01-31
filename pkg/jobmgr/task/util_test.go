// Copyright (c) 2019 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package task

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	mesos "github.com/uber/peloton/.gen/mesos/v1"
	"github.com/uber/peloton/.gen/peloton/api/v0/job"
	"github.com/uber/peloton/.gen/peloton/api/v0/peloton"
	"github.com/uber/peloton/.gen/peloton/api/v0/task"
	"github.com/uber/peloton/.gen/peloton/private/hostmgr/hostsvc"
	host_mocks "github.com/uber/peloton/.gen/peloton/private/hostmgr/hostsvc/mocks"

	"github.com/uber/peloton/pkg/common"
	"github.com/uber/peloton/pkg/common/util"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/yarpc/yarpcerrors"
	"golang.org/x/time/rate"
)

const (
	testSecretPath = "/tmp/secret"
	testSecretStr  = "top-secret-token"

	randomErrorStr = "random error"

	// task completes in 1 minute for the sake of this test
	taskStartTime      = "2017-01-02T15:04:00.456789016Z"
	taskCompletionTime = "2017-01-02T15:05:00.456789016Z"
)

type JobmgrTaskUtilTestSuite struct {
	suite.Suite
	ctrl        *gomock.Controller
	ctx         context.Context
	mockHostMgr *host_mocks.MockInternalHostServiceYARPCClient
	jobID       string
	instanceID  int32
	mesosTaskID string
	taskInfo    *task.TaskInfo
}

func (suite *JobmgrTaskUtilTestSuite) TearDownTest() {
	suite.ctrl.Finish()
}

// TestJobmgrTaskUtilTestSuite tests functions covered in jobmgr/task/util.go
func TestJobmgrTaskUtilTestSuite(t *testing.T) {
	suite.Run(t, new(JobmgrTaskUtilTestSuite))
}

func (suite *JobmgrTaskUtilTestSuite) SetupTest() {
	suite.ctrl = gomock.NewController(suite.T())
	suite.ctx = context.Background()
	suite.mockHostMgr = host_mocks.NewMockInternalHostServiceYARPCClient(suite.ctrl)
	suite.jobID = "af647b98-0ae0-4dac-be42-c74a524dfe44"
	suite.instanceID = 89
	suite.mesosTaskID = fmt.Sprintf(
		"%s-%d-%s",
		suite.jobID,
		suite.instanceID,
		uuid.New())
	suite.taskInfo = &task.TaskInfo{
		Runtime: &task.RuntimeInfo{
			MesosTaskId: &mesos.TaskID{Value: &suite.mesosTaskID},
			AgentID:     &mesos.AgentID{Value: &suite.mesosTaskID},
			State:       task.TaskState_RUNNING,
		},
		Config:     &task.TaskConfig{},
		InstanceId: uint32(0),
		JobId:      &peloton.JobID{Value: suite.jobID},
	}
}

// TestKillStateful tests when kill a stateful orphan
func (suite *JobmgrTaskUtilTestSuite) TestKillStateful() {
	suite.taskInfo.Config = &task.TaskConfig{
		Volume: &task.PersistentVolumeConfig{
			ContainerPath: "/A/B/C",
			SizeMB:        1024,
		},
	}
	suite.taskInfo.Runtime.VolumeID = &peloton.VolumeID{Value: "peloton_id"}
	err := KillOrphanTask(suite.ctx, suite.mockHostMgr, suite.taskInfo)
	suite.Nil(err)
}

// TestKillOrphanTaskSuccessStateKilled tests when kill a orphan task which is
// already in KILLED state.
func (suite *JobmgrTaskUtilTestSuite) TestKillOrphanTaskSuccessStateKilled() {
	suite.taskInfo.Runtime.State = task.TaskState_KILLED
	err := KillOrphanTask(suite.ctx, suite.mockHostMgr, suite.taskInfo)
	suite.Nil(err)
}

func (suite *JobmgrTaskUtilTestSuite,
) buildKillTasksReq() *hostsvc.KillTasksRequest {
	return &hostsvc.KillTasksRequest{
		TaskIds: []*mesos.TaskID{{Value: &suite.mesosTaskID}},
	}
}

// TestKillOrphanTaskSuccessStateRunning tests killing orphan task which is in
// RUNNING state
func (suite *JobmgrTaskUtilTestSuite) TestKillOrphanTaskSuccessStateRunning() {
	suite.mockHostMgr.EXPECT().
		KillTasks(gomock.Any(), suite.buildKillTasksReq())
	suite.taskInfo.Runtime.State = task.TaskState_RUNNING
	err := KillOrphanTask(suite.ctx, suite.mockHostMgr, suite.taskInfo)
	suite.Nil(err)
}

// TestKillOrphanTaskSuccessStateKilling tests killing orphan task which is in
// KILLING state
func (suite *JobmgrTaskUtilTestSuite) TestKillOrphanTaskSuccessStateKilling() {
	// simulate ShutdownMesosExecutor success for KILLING state
	suite.taskInfo.Runtime.State = task.TaskState_KILLING
	suite.mockHostMgr.EXPECT().ShutdownExecutors(gomock.Any(), gomock.Any()).
		Return(nil, nil)
	err := KillOrphanTask(suite.ctx, suite.mockHostMgr, suite.taskInfo)
	suite.Nil(err)
}

// TestKillOrphanTaskSuccessNoTaskInfo tests killing orphan task with taskInfo
// as nil
func (suite *JobmgrTaskUtilTestSuite) TestKillOrphanTaskSuccessNoTaskInfo() {
	err := KillOrphanTask(suite.ctx, suite.mockHostMgr, &task.TaskInfo{})
	suite.Nil(err)
}

// TestKillOrphanTaskRunning tests failure scenarios for KillOrphanTask when
// task is in RUNNING state
func (suite *JobmgrTaskUtilTestSuite) TestKillOrphanTaskRunning() {
	// simulate KillTasks failure for RUNNING state
	suite.taskInfo.Runtime.State = task.TaskState_RUNNING
	suite.mockHostMgr.EXPECT().
		KillTasks(gomock.Any(), suite.buildKillTasksReq()).
		Return(nil, errors.New(randomErrorStr))
	err := KillOrphanTask(suite.ctx, suite.mockHostMgr, suite.taskInfo)
	suite.Error(err)
}

// TestKillOrphanTaskKilling tests failure scenarios for KillOrphanTask when
// task is in KILLING state
func (suite *JobmgrTaskUtilTestSuite) TestKillOrphanTaskKilling() {
	// simulate ShutdownMesosExecutor failure for KILLING state
	suite.taskInfo.Runtime.State = task.TaskState_KILLING
	suite.mockHostMgr.EXPECT().ShutdownExecutors(gomock.Any(), gomock.Any()).
		Return(nil, errors.New(randomErrorStr))
	err := KillOrphanTask(suite.ctx, suite.mockHostMgr, suite.taskInfo)
	suite.Error(err)
}

// TestKillTaskInvalidTaskIDs tests InvalidTaskIDs error in KillTask
func (suite *JobmgrTaskUtilTestSuite) TestKillTaskInvalidTaskIDs() {
	taskID := &mesos.TaskID{Value: &suite.mesosTaskID}

	// Simulate InvalidTaskIDs error
	resp := &hostsvc.KillTasksResponse{
		Error: &hostsvc.KillTasksResponse_Error{
			InvalidTaskIDs: &hostsvc.InvalidTaskIDs{
				Message: randomErrorStr,
			},
		},
	}
	suite.mockHostMgr.EXPECT().KillTasks(
		gomock.Any(), suite.buildKillTasksReq()).Return(resp, nil)
	err := KillTask(suite.ctx, suite.mockHostMgr, taskID, "", nil)
	suite.Error(err)
	suite.Equal(err.Error(), randomErrorStr)
}

// TestKillTaskKillFailure tests KillFailure error in KillTask
func (suite *JobmgrTaskUtilTestSuite) TestKillTaskKillFailure() {
	taskID := &mesos.TaskID{Value: &suite.mesosTaskID}

	// Simulate KillFailure error
	resp := &hostsvc.KillTasksResponse{
		Error: &hostsvc.KillTasksResponse_Error{
			KillFailure: &hostsvc.KillFailure{
				Message: randomErrorStr,
			},
		},
	}
	suite.mockHostMgr.EXPECT().KillTasks(
		gomock.Any(), suite.buildKillTasksReq()).Return(resp, nil)
	err := KillTask(suite.ctx, suite.mockHostMgr, taskID, "", nil)
	suite.Error(err)
	suite.Equal(err.Error(), randomErrorStr)
}

// TestKillTaskRateLimit tests task kill fails due to rate limit reached
func (suite *JobmgrTaskUtilTestSuite) TestKillTaskRateLimit() {
	taskID := &mesos.TaskID{Value: &suite.mesosTaskID}
	err := KillTask(suite.ctx, suite.mockHostMgr, taskID, "", rate.NewLimiter(0, 0))
	suite.Error(err)
	suite.True(yarpcerrors.IsResourceExhausted(err))
}

func (suite *JobmgrTaskUtilTestSuite,
) buildShutdownExecutorsReq() *hostsvc.ShutdownExecutorsRequest {
	return &hostsvc.ShutdownExecutorsRequest{
		Executors: []*hostsvc.ExecutorOnAgent{
			{
				ExecutorId: &mesos.ExecutorID{Value: &suite.mesosTaskID},
				AgentId:    &mesos.AgentID{Value: &suite.mesosTaskID},
			},
		},
	}
}

// TestShutdownExecutorShutdownFailure tests ShutdownFailure error in
// ShutdownMesosExecutor
func (suite *JobmgrTaskUtilTestSuite) TestShutdownExecutorShutdownFailure() {
	// Simulate ShutdownFailure error
	resp := &hostsvc.ShutdownExecutorsResponse{
		Error: &hostsvc.ShutdownExecutorsResponse_Error{
			ShutdownFailure: &hostsvc.ShutdownFailure{
				Message: randomErrorStr,
			},
		},
	}
	suite.mockHostMgr.EXPECT().ShutdownExecutors(
		suite.ctx, suite.buildShutdownExecutorsReq()).Return(resp, nil)
	err := ShutdownMesosExecutor(
		suite.ctx,
		suite.mockHostMgr,
		&mesos.TaskID{Value: &suite.mesosTaskID},
		&mesos.AgentID{Value: &suite.mesosTaskID},
		nil)
	suite.Error(err)
	suite.Equal(err.Error(), randomErrorStr)
}

// TestExecutorShutdownRateLimit tests executor shutdown fails due to
// rate limit
func (suite *JobmgrTaskUtilTestSuite) TestExecutorShutdownRateLimit() {
	err := ShutdownMesosExecutor(
		suite.ctx,
		suite.mockHostMgr,
		&mesos.TaskID{Value: &suite.mesosTaskID},
		&mesos.AgentID{Value: &suite.mesosTaskID},
		rate.NewLimiter(0, 0))
	suite.Error(err)
	suite.True(yarpcerrors.IsResourceExhausted(err))
}

// TestShutdownExecutorInvalidExecutors tests InvalidExecutors error in
// ShutdownMesosExecutor
func (suite *JobmgrTaskUtilTestSuite) TestShutdownExecutorInvalidExecutors() {

	// Simulate InvalidExecutors error
	resp := &hostsvc.ShutdownExecutorsResponse{
		Error: &hostsvc.ShutdownExecutorsResponse_Error{
			InvalidExecutors: &hostsvc.InvalidExecutors{
				Message: randomErrorStr,
			},
		},
	}
	suite.mockHostMgr.EXPECT().ShutdownExecutors(
		suite.ctx, suite.buildShutdownExecutorsReq()).Return(resp, nil)
	err := ShutdownMesosExecutor(
		suite.ctx,
		suite.mockHostMgr,
		&mesos.TaskID{Value: &suite.mesosTaskID},
		&mesos.AgentID{Value: &suite.mesosTaskID},
		nil)
	suite.Error(err)
	suite.Equal(err.Error(), randomErrorStr)
}

// TestCreateInitializingTask tests CreateInitializingTask
func (suite *JobmgrTaskUtilTestSuite) TestCreateInitializingTask() {
	runtime := CreateInitializingTask(&peloton.JobID{Value: suite.jobID},
		uint32(suite.instanceID), &job.JobConfig{})
	suite.Equal(runtime.GetState(), task.TaskState_INITIALIZED)
	suite.Equal(runtime.GetGoalState(), task.TaskState_SUCCEEDED)
	suite.Equal(runtime.GetMesosTaskId(), runtime.GetDesiredMesosTaskId())
	suite.Equal(runtime.GetHealthy(), task.HealthState_DISABLED)
	suite.NotEmpty(runtime.GetMesosTaskId())
	suite.NotEmpty(runtime.GetDesiredMesosTaskId())
}

func (suite *JobmgrTaskUtilTestSuite) TestCreateInitializingTaskWithHealthCheck() {
	taskConfigWithHealth := task.TaskConfig{
		HealthCheck: &task.HealthCheckConfig{
			Enabled:                true,
			InitialIntervalSecs:    10,
			IntervalSecs:           10,
			MaxConsecutiveFailures: 5,
			TimeoutSecs:            5,
		},
	}
	jobConfig := job.JobConfig{
		DefaultConfig: &taskConfigWithHealth,
	}
	runtime := CreateInitializingTask(&peloton.JobID{Value: suite.jobID},
		uint32(suite.instanceID), &jobConfig)
	suite.Equal(runtime.GetHealthy(), task.HealthState_HEALTH_UNKNOWN)
	suite.Equal(runtime.GetMesosTaskId(), runtime.GetDesiredMesosTaskId())
	suite.NotEmpty(runtime.GetMesosTaskId())
	suite.NotEmpty(runtime.GetDesiredMesosTaskId())
}

func (suite *JobmgrTaskUtilTestSuite) TestCreateInitializingTaskWithHealthCheckDisabled() {
	taskConfigWithHealth := task.TaskConfig{
		HealthCheck: &task.HealthCheckConfig{
			Enabled:                false,
			InitialIntervalSecs:    10,
			IntervalSecs:           10,
			MaxConsecutiveFailures: 5,
			TimeoutSecs:            5,
		},
	}
	jobConfig := job.JobConfig{
		DefaultConfig: &taskConfigWithHealth,
	}
	runtime := CreateInitializingTask(&peloton.JobID{Value: suite.jobID},
		uint32(suite.instanceID), &jobConfig)
	suite.Equal(runtime.GetHealthy(), task.HealthState_DISABLED)
	suite.Equal(runtime.GetMesosTaskId(), runtime.GetDesiredMesosTaskId())
	suite.NotEmpty(runtime.GetMesosTaskId())
	suite.NotEmpty(runtime.GetDesiredMesosTaskId())
}

// TestGetDefaultTaskGoalState tests GetDefaultTaskGoalState
func (suite *JobmgrTaskUtilTestSuite) TestGetDefaultTaskGoalState() {
	state := GetDefaultTaskGoalState(job.JobType_SERVICE)
	suite.Equal(state, task.TaskState_RUNNING)

	state = GetDefaultTaskGoalState(job.JobType_BATCH)
	suite.Equal(state, task.TaskState_SUCCEEDED)

}

// TestCreateSecretProto tests if CreateSecretProto creates a secret protobuf
// message from given secret path and data
func (suite *JobmgrTaskUtilTestSuite) TestCreateSecretProto() {
	id := uuid.New()
	secret := CreateSecretProto(id, testSecretPath, []byte(testSecretStr))
	suite.Equal(secret.GetPath(), testSecretPath)
	suite.Equal(secret.GetId().GetValue(), id)
	suite.Equal(secret.GetValue().GetData(),
		[]byte(base64.StdEncoding.EncodeToString([]byte(testSecretStr))))
}

// TestCreateSecretsFromVolumes tests building secret proto from secret volumes
func (suite *JobmgrTaskUtilTestSuite) TestCreateSecretsFromVolumes() {
	id := uuid.New()
	secrets := CreateSecretsFromVolumes(
		[]*mesos.Volume{util.CreateSecretVolume(testSecretPath, id)})
	suite.Equal(len(secrets), 1)
	suite.Equal(secrets[0].GetPath(), testSecretPath)
	suite.Nil(secrets[0].GetValue().GetData())
	suite.Equal(secrets[0].GetId().GetValue(), id)
}

// TestCreateEmptyResourceUsageMap tests creating empty resource usage map
func (suite *JobmgrTaskUtilTestSuite) TestCreateEmptyResourceUsageMap() {
	suite.Equal(map[string]float64{
		common.CPU:    float64(0),
		common.GPU:    float64(0),
		common.MEMORY: float64(0)}, CreateEmptyResourceUsageMap())
}

// TestCreateResourceUsageMap tests creating resource usage stats map
func (suite *JobmgrTaskUtilTestSuite) TestCreateResourceUsageMap() {
	resourceConfig := &task.ResourceConfig{
		CpuLimit:   float64(0.1),
		MemLimitMb: float64(0.2),
		GpuLimit:   float64(0),
	}

	// Task completes in 1 minute as per our test.
	// So the CPU usage should be 0.1 x 60 = 6,
	// GPU usage should be 0 x 60 = 0 and Memory usage should be 0.2 x 60 = 12
	rMap, err := CreateResourceUsageMap(
		resourceConfig, taskStartTime, taskCompletionTime)
	suite.Nil(err)
	suite.Equal(map[string]float64{
		common.CPU:    float64(6),
		common.GPU:    float64(0),
		common.MEMORY: float64(12)}, rMap)
}

// TestCreateResourceUsageMapError tests error cases in CreateResourceUsageMap
func (suite *JobmgrTaskUtilTestSuite) TestCreateResourceUsageMapError() {
	resourceConfig := &task.ResourceConfig{
		CpuLimit:   float64(0.1),
		MemLimitMb: float64(0.2),
		GpuLimit:   float64(0),
	}

	// startTime is "", the resource map should have 0 value for all resources
	rMap, err := CreateResourceUsageMap(
		resourceConfig, "", taskCompletionTime)
	suite.Nil(err)
	suite.Equal(CreateEmptyResourceUsageMap(), rMap)

	// start time is not valid
	rMap, err = CreateResourceUsageMap(
		resourceConfig, "not-valid-time", taskCompletionTime)
	suite.Error(err)
	suite.Nil(rMap)

	//	completion time is not valid
	rMap, err = CreateResourceUsageMap(
		resourceConfig, taskStartTime, "not-valid-time")
	suite.Error(err)
	suite.Nil(rMap)
}
