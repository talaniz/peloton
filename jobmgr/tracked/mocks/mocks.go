// Code generated by MockGen. DO NOT EDIT.
// Source: code.uber.internal/infra/peloton/jobmgr/tracked (interfaces: Manager,Job,Task)

package mocks

import (
	context "context"
	reflect "reflect"
	time "time"

	peloton "code.uber.internal/infra/peloton/.gen/peloton/api/peloton"
	task "code.uber.internal/infra/peloton/.gen/peloton/api/task"
	tracked "code.uber.internal/infra/peloton/jobmgr/tracked"
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
func (_m *MockManager) EXPECT() *MockManagerMockRecorder {
	return _m.recorder
}

// GetAllJobs mocks base method
func (_m *MockManager) GetAllJobs() map[string]tracked.Job {
	ret := _m.ctrl.Call(_m, "GetAllJobs")
	ret0, _ := ret[0].(map[string]tracked.Job)
	return ret0
}

// GetAllJobs indicates an expected call of GetAllJobs
func (_mr *MockManagerMockRecorder) GetAllJobs() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GetAllJobs", reflect.TypeOf((*MockManager)(nil).GetAllJobs))
}

// GetJob mocks base method
func (_m *MockManager) GetJob(_param0 *peloton.JobID) tracked.Job {
	ret := _m.ctrl.Call(_m, "GetJob", _param0)
	ret0, _ := ret[0].(tracked.Job)
	return ret0
}

// GetJob indicates an expected call of GetJob
func (_mr *MockManagerMockRecorder) GetJob(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GetJob", reflect.TypeOf((*MockManager)(nil).GetJob), arg0)
}

// ScheduleTask mocks base method
func (_m *MockManager) ScheduleTask(_param0 tracked.Task, _param1 time.Time) {
	_m.ctrl.Call(_m, "ScheduleTask", _param0, _param1)
}

// ScheduleTask indicates an expected call of ScheduleTask
func (_mr *MockManagerMockRecorder) ScheduleTask(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "ScheduleTask", reflect.TypeOf((*MockManager)(nil).ScheduleTask), arg0, arg1)
}

// SetTask mocks base method
func (_m *MockManager) SetTask(_param0 *peloton.JobID, _param1 uint32, _param2 *task.RuntimeInfo) {
	_m.ctrl.Call(_m, "SetTask", _param0, _param1, _param2)
}

// SetTask indicates an expected call of SetTask
func (_mr *MockManagerMockRecorder) SetTask(arg0, arg1, arg2 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "SetTask", reflect.TypeOf((*MockManager)(nil).SetTask), arg0, arg1, arg2)
}

// Start mocks base method
func (_m *MockManager) Start() {
	_m.ctrl.Call(_m, "Start")
}

// Start indicates an expected call of Start
func (_mr *MockManagerMockRecorder) Start() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "Start", reflect.TypeOf((*MockManager)(nil).Start))
}

// Stop mocks base method
func (_m *MockManager) Stop() {
	_m.ctrl.Call(_m, "Stop")
}

// Stop indicates an expected call of Stop
func (_mr *MockManagerMockRecorder) Stop() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "Stop", reflect.TypeOf((*MockManager)(nil).Stop))
}

// UpdateTaskRuntime mocks base method
func (_m *MockManager) UpdateTaskRuntime(_param0 context.Context, _param1 *peloton.JobID, _param2 uint32, _param3 *task.RuntimeInfo) error {
	ret := _m.ctrl.Call(_m, "UpdateTaskRuntime", _param0, _param1, _param2, _param3)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateTaskRuntime indicates an expected call of UpdateTaskRuntime
func (_mr *MockManagerMockRecorder) UpdateTaskRuntime(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "UpdateTaskRuntime", reflect.TypeOf((*MockManager)(nil).UpdateTaskRuntime), arg0, arg1, arg2, arg3)
}

// WaitForScheduledTask mocks base method
func (_m *MockManager) WaitForScheduledTask(_param0 <-chan struct{}) tracked.Task {
	ret := _m.ctrl.Call(_m, "WaitForScheduledTask", _param0)
	ret0, _ := ret[0].(tracked.Task)
	return ret0
}

// WaitForScheduledTask indicates an expected call of WaitForScheduledTask
func (_mr *MockManagerMockRecorder) WaitForScheduledTask(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "WaitForScheduledTask", reflect.TypeOf((*MockManager)(nil).WaitForScheduledTask), arg0)
}

// MockJob is a mock of Job interface
type MockJob struct {
	ctrl     *gomock.Controller
	recorder *MockJobMockRecorder
}

// MockJobMockRecorder is the mock recorder for MockJob
type MockJobMockRecorder struct {
	mock *MockJob
}

// NewMockJob creates a new mock instance
func NewMockJob(ctrl *gomock.Controller) *MockJob {
	mock := &MockJob{ctrl: ctrl}
	mock.recorder = &MockJobMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (_m *MockJob) EXPECT() *MockJobMockRecorder {
	return _m.recorder
}

// GetAllTasks mocks base method
func (_m *MockJob) GetAllTasks() map[uint32]tracked.Task {
	ret := _m.ctrl.Call(_m, "GetAllTasks")
	ret0, _ := ret[0].(map[uint32]tracked.Task)
	return ret0
}

// GetAllTasks indicates an expected call of GetAllTasks
func (_mr *MockJobMockRecorder) GetAllTasks() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GetAllTasks", reflect.TypeOf((*MockJob)(nil).GetAllTasks))
}

// GetTask mocks base method
func (_m *MockJob) GetTask(_param0 uint32) tracked.Task {
	ret := _m.ctrl.Call(_m, "GetTask", _param0)
	ret0, _ := ret[0].(tracked.Task)
	return ret0
}

// GetTask indicates an expected call of GetTask
func (_mr *MockJobMockRecorder) GetTask(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GetTask", reflect.TypeOf((*MockJob)(nil).GetTask), arg0)
}

// ID mocks base method
func (_m *MockJob) ID() *peloton.JobID {
	ret := _m.ctrl.Call(_m, "ID")
	ret0, _ := ret[0].(*peloton.JobID)
	return ret0
}

// ID indicates an expected call of ID
func (_mr *MockJobMockRecorder) ID() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "ID", reflect.TypeOf((*MockJob)(nil).ID))
}

// MockTask is a mock of Task interface
type MockTask struct {
	ctrl     *gomock.Controller
	recorder *MockTaskMockRecorder
}

// MockTaskMockRecorder is the mock recorder for MockTask
type MockTaskMockRecorder struct {
	mock *MockTask
}

// NewMockTask creates a new mock instance
func NewMockTask(ctrl *gomock.Controller) *MockTask {
	mock := &MockTask{ctrl: ctrl}
	mock.recorder = &MockTaskMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (_m *MockTask) EXPECT() *MockTaskMockRecorder {
	return _m.recorder
}

// CurrentState mocks base method
func (_m *MockTask) CurrentState() tracked.State {
	ret := _m.ctrl.Call(_m, "CurrentState")
	ret0, _ := ret[0].(tracked.State)
	return ret0
}

// CurrentState indicates an expected call of CurrentState
func (_mr *MockTaskMockRecorder) CurrentState() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "CurrentState", reflect.TypeOf((*MockTask)(nil).CurrentState))
}

// GetRunTime mocks base method
func (_m *MockTask) GetRunTime() *task.RuntimeInfo {
	ret := _m.ctrl.Call(_m, "GetRunTime")
	ret0, _ := ret[0].(*task.RuntimeInfo)
	return ret0
}

// GetRunTime indicates an expected call of GetRunTime
func (_mr *MockTaskMockRecorder) GetRunTime() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GetRunTime", reflect.TypeOf((*MockTask)(nil).GetRunTime))
}

// GoalState mocks base method
func (_m *MockTask) GoalState() tracked.State {
	ret := _m.ctrl.Call(_m, "GoalState")
	ret0, _ := ret[0].(tracked.State)
	return ret0
}

// GoalState indicates an expected call of GoalState
func (_mr *MockTaskMockRecorder) GoalState() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GoalState", reflect.TypeOf((*MockTask)(nil).GoalState))
}

// ID mocks base method
func (_m *MockTask) ID() uint32 {
	ret := _m.ctrl.Call(_m, "ID")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// ID indicates an expected call of ID
func (_mr *MockTaskMockRecorder) ID() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "ID", reflect.TypeOf((*MockTask)(nil).ID))
}

// Job mocks base method
func (_m *MockTask) Job() tracked.Job {
	ret := _m.ctrl.Call(_m, "Job")
	ret0, _ := ret[0].(tracked.Job)
	return ret0
}

// Job indicates an expected call of Job
func (_mr *MockTaskMockRecorder) Job() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "Job", reflect.TypeOf((*MockTask)(nil).Job))
}

// LastAction mocks base method
func (_m *MockTask) LastAction() (tracked.TaskAction, time.Time) {
	ret := _m.ctrl.Call(_m, "LastAction")
	ret0, _ := ret[0].(tracked.TaskAction)
	ret1, _ := ret[1].(time.Time)
	return ret0, ret1
}

// LastAction indicates an expected call of LastAction
func (_mr *MockTaskMockRecorder) LastAction() *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "LastAction", reflect.TypeOf((*MockTask)(nil).LastAction))
}

// RunAction mocks base method
func (_m *MockTask) RunAction(_param0 context.Context, _param1 tracked.TaskAction) (bool, error) {
	ret := _m.ctrl.Call(_m, "RunAction", _param0, _param1)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RunAction indicates an expected call of RunAction
func (_mr *MockTaskMockRecorder) RunAction(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "RunAction", reflect.TypeOf((*MockTask)(nil).RunAction), arg0, arg1)
}
