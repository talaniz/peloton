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

package event

import (
	"context"
	"strings"
	"time"

	"github.com/uber/peloton/.gen/mesos/v1"
	pbjob "github.com/uber/peloton/.gen/peloton/api/v0/job"
	pb_task "github.com/uber/peloton/.gen/peloton/api/v0/task"
	"github.com/uber/peloton/.gen/peloton/api/v0/volume"
	pb_eventstream "github.com/uber/peloton/.gen/peloton/private/eventstream"
	"github.com/uber/peloton/.gen/peloton/private/hostmgr/hostsvc"

	"github.com/uber/peloton/pkg/common"
	"github.com/uber/peloton/pkg/common/eventstream"
	"github.com/uber/peloton/pkg/common/util"
	"github.com/uber/peloton/pkg/jobmgr/cached"
	"github.com/uber/peloton/pkg/jobmgr/goalstate"
	jobmgr_task "github.com/uber/peloton/pkg/jobmgr/task"
	taskutil "github.com/uber/peloton/pkg/jobmgr/util/task"
	"github.com/uber/peloton/pkg/storage"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/uber-go/tally"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/yarpcerrors"
)

const (
	// Mesos event message that indicates duplicate task ID
	_msgMesosDuplicateID = "Task has duplicate ID"

	// _numOrphanTaskKillAttempts is number of attempts to
	// kill orphan task in case of error from host manager
	_numOrphanTaskKillAttempts = 3

	// _waitForRetryOnError is the time between successive retries
	// to kill orphan task in case of error from host manager
	_waitForRetryOnErrorOrphanTaskKill = 5 * time.Millisecond
)

// Declare a Now function so that we can mock it in unit tests.
var now = time.Now

// StatusUpdate is the interface for task status updates
type StatusUpdate interface {
	Start()
	Stop()
}

// Listener is the interface for StatusUpdate listener
type Listener interface {
	eventstream.EventHandler

	Start()
	Stop()
}

// StatusUpdate reads and processes the task state change events from HM
type statusUpdate struct {
	jobStore        storage.JobStore
	taskStore       storage.TaskStore
	volumeStore     storage.PersistentVolumeStore
	eventClients    map[string]*eventstream.Client
	hostmgrClient   hostsvc.InternalHostServiceYARPCClient
	applier         *asyncEventProcessor
	jobFactory      cached.JobFactory
	goalStateDriver goalstate.Driver
	listeners       []Listener
	rootCtx         context.Context
	metrics         *Metrics
}

// NewTaskStatusUpdate creates a statusUpdate
func NewTaskStatusUpdate(
	d *yarpc.Dispatcher,
	jobStore storage.JobStore,
	taskStore storage.TaskStore,
	volumeStore storage.PersistentVolumeStore,
	jobFactory cached.JobFactory,
	goalStateDriver goalstate.Driver,
	listeners []Listener,
	parentScope tally.Scope) StatusUpdate {

	statusUpdater := &statusUpdate{
		jobStore:        jobStore,
		taskStore:       taskStore,
		volumeStore:     volumeStore,
		rootCtx:         context.Background(),
		metrics:         NewMetrics(parentScope.SubScope("status_updater")),
		eventClients:    make(map[string]*eventstream.Client),
		jobFactory:      jobFactory,
		goalStateDriver: goalStateDriver,
		listeners:       listeners,
		hostmgrClient:   hostsvc.NewInternalHostServiceYARPCClient(d.ClientConfig(common.PelotonHostManager)),
	}
	// TODO: add config for BucketEventProcessor
	statusUpdater.applier = newBucketEventProcessor(statusUpdater, 100, 10000)

	eventClient := eventstream.NewEventStreamClient(
		d,
		common.PelotonJobManager,
		common.PelotonHostManager,
		statusUpdater,
		parentScope.SubScope("HostmgrEventStreamClient"))
	statusUpdater.eventClients[common.PelotonHostManager] = eventClient

	eventClientRM := eventstream.NewEventStreamClient(
		d,
		common.PelotonJobManager,
		common.PelotonResourceManager,
		statusUpdater,
		parentScope.SubScope("ResmgrEventStreamClient"))
	statusUpdater.eventClients[common.PelotonResourceManager] = eventClientRM
	return statusUpdater
}

// OnEvent is the callback function notifying an event
func (p *statusUpdate) OnEvent(event *pb_eventstream.Event) {
	log.WithField("event_offset", event.Offset).Debug("JobMgr receiving event")
	p.applier.addEvent(event)
}

// GetEventProgress returns the progress of the event progressing
func (p *statusUpdate) GetEventProgress() uint64 {
	return p.applier.GetEventProgress()
}

// ProcessStatusUpdate processes the actual task status
func (p *statusUpdate) ProcessStatusUpdate(ctx context.Context, event *pb_eventstream.Event) error {
	var currTaskResourceUsage map[string]float64
	updateEvent, err := convertEvent(event)
	if err != nil {
		return err
	}

	p.logTaskMetrics(updateEvent)

	isOrphanTask, taskInfo, err := p.isOrphanTaskEvent(ctx, updateEvent)
	if err != nil {
		return err
	}

	if isOrphanTask {
		p.metrics.SkipOrphanTasksTotal.Inc(1)
		taskInfo := &pb_task.TaskInfo{
			Runtime: &pb_task.RuntimeInfo{
				State:       updateEvent.state,
				MesosTaskId: event.MesosTaskStatus.GetTaskId(),
				AgentID:     event.MesosTaskStatus.GetAgentId(),
			},
		}

		// Kill the orphan task
		for i := 0; i < _numOrphanTaskKillAttempts; i++ {
			err = jobmgr_task.KillOrphanTask(ctx, p.hostmgrClient, taskInfo)
			if err == nil {
				return nil
			}
			time.Sleep(_waitForRetryOnErrorOrphanTaskKill)
		}
		return nil
	}

	// whether to skip or not if instance state is similar before and after
	if isDuplicateStateUpdate(
		taskInfo,
		event,
		updateEvent.state) {
		return nil
	}

	if updateEvent.state == pb_task.TaskState_RUNNING &&
		taskInfo.GetConfig().GetVolume() != nil &&
		len(taskInfo.GetRuntime().GetVolumeID().GetValue()) != 0 {
		// Update volume state to be CREATED upon task RUNNING.
		if err := p.updatePersistentVolumeState(ctx, taskInfo); err != nil {
			return err
		}
	}

	newRuntime := proto.Clone(taskInfo.GetRuntime()).(*pb_task.RuntimeInfo)

	// Persist the reason and message for mesos updates
	newRuntime.Message = updateEvent.statusMsg
	newRuntime.Reason = ""

	// Persist healthy field if health check is enabled
	if taskInfo.GetConfig().GetHealthCheck() != nil {
		reason := event.GetMesosTaskStatus().GetReason()
		healthy := event.GetMesosTaskStatus().GetHealthy()
		p.persistHealthyField(updateEvent.state, reason, healthy, newRuntime)
	}

	// Update FailureCount
	updateFailureCount(updateEvent.state, taskInfo.GetRuntime(), newRuntime)

	switch updateEvent.state {
	case pb_task.TaskState_FAILED:
		reason := event.GetMesosTaskStatus().GetReason()
		msg := event.GetMesosTaskStatus().GetMessage()
		if reason == mesos_v1.TaskStatus_REASON_TASK_INVALID &&
			strings.Contains(msg, _msgMesosDuplicateID) {
			log.WithField("task_id", updateEvent.taskID).
				Info("ignoring duplicate task id failure")
			return nil
		}
		newRuntime.Reason = reason.String()
		newRuntime.State = updateEvent.state
		newRuntime.Message = msg
		termStatus := &pb_task.TerminationStatus{
			Reason: pb_task.TerminationStatus_TERMINATION_STATUS_REASON_FAILED,
		}
		if code, err := taskutil.GetExitStatusFromMessage(msg); err == nil {
			termStatus.ExitCode = code
		} else if yarpcerrors.IsNotFound(err) == false {
			log.WithField("task_id", updateEvent.taskID).
				WithField("error", err).
				Debug("Failed to extract exit status from message")
		}
		if sig, err := taskutil.GetSignalFromMessage(msg); err == nil {
			termStatus.Signal = sig
		} else if yarpcerrors.IsNotFound(err) == false {
			log.WithField("task_id", updateEvent.taskID).
				WithField("error", err).
				Debug("Failed to extract termination signal from message")
		}
		newRuntime.TerminationStatus = termStatus

	case pb_task.TaskState_LOST:
		newRuntime.Reason = event.GetMesosTaskStatus().GetReason().String()
		if util.IsPelotonStateTerminal(taskInfo.GetRuntime().GetState()) {
			// Skip LOST status update if current state is terminal state.
			log.WithFields(log.Fields{
				"task_id":           updateEvent.taskID,
				"db_task_runtime":   taskInfo.GetRuntime(),
				"task_status_event": event.GetMesosTaskStatus(),
			}).Debug("skip reschedule lost task as it is already in terminal state")
			return nil
		}
		if taskInfo.GetRuntime().GetGoalState() == pb_task.TaskState_KILLED {
			// Do not take any action for killed tasks, just mark it killed.
			// Same message will go to resource manager which will release the placement.
			log.WithFields(log.Fields{
				"task_id":           updateEvent.taskID,
				"db_task_runtime":   taskInfo.GetRuntime(),
				"task_status_event": event.GetMesosTaskStatus(),
			}).Debug("mark stopped task as killed due to LOST")
			newRuntime.State = pb_task.TaskState_KILLED
			newRuntime.Message = "Stopped task LOST event: " + updateEvent.statusMsg
			break
		}

		if taskInfo.GetConfig().GetVolume() != nil &&
			len(taskInfo.GetRuntime().GetVolumeID().GetValue()) != 0 {
			// Do not reschedule stateful task. Storage layer will decide
			// whether to start or replace this task.
			newRuntime.State = pb_task.TaskState_LOST
			break
		}

		log.WithFields(log.Fields{
			"task_id":           updateEvent.taskID,
			"db_task_runtime":   taskInfo.GetRuntime(),
			"task_status_event": event.GetMesosTaskStatus(),
		}).Info("reschedule lost task if needed")

		newRuntime.State = pb_task.TaskState_LOST
		newRuntime.Message = "Task LOST: " + updateEvent.statusMsg
		newRuntime.Reason = event.GetMesosTaskStatus().GetReason().String()

		// Calculate resource usage for TaskState_LOST using time.Now() as
		// completion time
		currTaskResourceUsage = getCurrTaskResourceUsage(
			updateEvent.taskID, updateEvent.state, taskInfo.GetConfig().GetResource(),
			taskInfo.GetRuntime().GetStartTime(),
			now().UTC().Format(time.RFC3339Nano))

	default:
		newRuntime.State = updateEvent.state
	}

	cachedJob := p.jobFactory.AddJob(taskInfo.GetJobId())
	// Update task start and completion timestamps
	if newRuntime.GetState() == pb_task.TaskState_RUNNING {
		if updateEvent.state != taskInfo.GetRuntime().GetState() {
			// StartTime is set at the time of first RUNNING event
			// CompletionTime may have been set (e.g. task has been set),
			// which could make StartTime larger than CompletionTime.
			// Reset CompletionTime every time a task transits to RUNNING state.
			newRuntime.StartTime = now().UTC().Format(time.RFC3339Nano)
			newRuntime.CompletionTime = ""
			// when task is RUNNING, reset the desired host field. Therefore,
			// the task would be scheduled onto a different host when the task
			// restarts (e.g due to health check or fail retry)
			newRuntime.DesiredHost = ""

			if len(taskInfo.GetRuntime().GetDesiredHost()) != 0 {
				p.metrics.TasksInPlacePlacementTotal.Inc(1)
				if taskInfo.GetRuntime().GetDesiredHost() == taskInfo.GetRuntime().GetHost() {
					p.metrics.TasksInPlacePlacementSuccess.Inc(1)
				}
			}
		}

	} else if util.IsPelotonStateTerminal(newRuntime.GetState()) &&
		cachedJob.GetJobType() == pbjob.JobType_BATCH {
		// only update resource count when a batch job is in terminal state
		completionTime := now().UTC().Format(time.RFC3339Nano)
		newRuntime.CompletionTime = completionTime

		currTaskResourceUsage = getCurrTaskResourceUsage(
			updateEvent.taskID, updateEvent.state, taskInfo.GetConfig().GetResource(),
			taskInfo.GetRuntime().GetStartTime(), completionTime)

		if len(currTaskResourceUsage) > 0 {
			// current task resource usage was updated by this event, so we should
			// add it to aggregated resource usage for the task and update runtime
			aggregateTaskResourceUsage := taskInfo.GetRuntime().GetResourceUsage()
			if len(aggregateTaskResourceUsage) > 0 {
				for k, v := range currTaskResourceUsage {
					aggregateTaskResourceUsage[k] += v
				}
				newRuntime.ResourceUsage = aggregateTaskResourceUsage
			}
		}
	} else if cachedJob.GetJobType() == pbjob.JobType_SERVICE {
		// for service job, reset resource usage
		currTaskResourceUsage = nil
		newRuntime.ResourceUsage = nil
	}

	// Update the task update times in job cache and then update the task runtime in cache and DB
	cachedJob.SetTaskUpdateTime(event.MesosTaskStatus.Timestamp)
	cachedTask, err := cachedJob.AddTask(ctx, taskInfo.GetInstanceId())
	if err != nil {
		return err
	}
	if _, err := cachedTask.CompareAndSetTask(ctx, newRuntime, cachedJob.GetJobType()); err != nil {
		log.WithError(err).
			WithFields(log.Fields{
				"task_id": updateEvent.taskID,
				"state":   updateEvent.state}).
			Error("Fail to update runtime for taskID")
		return err
	}

	// Enqueue task to goal state
	p.goalStateDriver.EnqueueTask(
		taskInfo.GetJobId(),
		taskInfo.GetInstanceId(),
		time.Now())
	// Enqueue job to goal state as well
	goalstate.EnqueueJobWithDefaultDelay(
		taskInfo.GetJobId(), p.goalStateDriver, cachedJob)

	// Update job's resource usage with the current task resource usage.
	// This is a noop in case currTaskResourceUsage is nil
	// This operation is not idempotent. So we will update job resource usage
	// in cache only after successfully updating task resource usage in DB
	// In case of errors in PatchTasks(), ProcessStatusUpdate will be retried
	// indefinitely until errors are resolved.
	cachedJob.UpdateResourceUsage(currTaskResourceUsage)
	return nil
}

type statusUpateEvent struct {
	taskID    string
	state     pb_task.TaskState
	statusMsg string

	isMesosStatus   bool
	mesosTaskStatus *mesos_v1.TaskStatus
}

// convertEvent converts pb_eventstream.Event to statusUpateEvent
// so it is easier for statusUpdate to process
func convertEvent(event *pb_eventstream.Event) (*statusUpateEvent, error) {
	var err error

	updateEvent := &statusUpateEvent{mesosTaskStatus: &mesos_v1.TaskStatus{}}
	if event.Type == pb_eventstream.Event_MESOS_TASK_STATUS {
		mesosTaskID := event.MesosTaskStatus.GetTaskId().GetValue()
		updateEvent.taskID, err = util.ParseTaskIDFromMesosTaskID(mesosTaskID)
		if err != nil {
			log.WithError(err).
				WithField("task_id", mesosTaskID).
				Error("Fail to parse taskID for mesostaskID")
			return nil, err
		}
		updateEvent.state = util.MesosStateToPelotonState(event.MesosTaskStatus.GetState())
		updateEvent.statusMsg = event.MesosTaskStatus.GetMessage()

		updateEvent.isMesosStatus = true
		updateEvent.mesosTaskStatus = event.MesosTaskStatus
		log.WithFields(log.Fields{
			"task_id": updateEvent.taskID,
			"state":   updateEvent.state.String(),
		}).Debug("Adding Mesos Event ")

	} else if event.Type == pb_eventstream.Event_PELOTON_TASK_EVENT {
		// Peloton task event is used for task status update from resmgr.
		updateEvent.taskID = event.PelotonTaskEvent.TaskId.Value
		updateEvent.state = event.PelotonTaskEvent.State
		updateEvent.statusMsg = event.PelotonTaskEvent.Message
		log.WithFields(log.Fields{
			"task_id": updateEvent.taskID,
			"state":   updateEvent.state.String(),
		}).Debug("Adding Peloton Event ")
	} else {
		log.WithFields(log.Fields{
			"task_id": updateEvent.taskID,
			"state":   updateEvent.state.String(),
		}).Error("Unknown Event ")
		return nil, errors.New("Unknown Event ")
	}
	return updateEvent, nil
}

// logTaskMetrics logs events metrics
func (p *statusUpdate) logTaskMetrics(event *statusUpateEvent) {
	// Update task state counter for non-reconcilication update.
	if event.isMesosStatus && event.mesosTaskStatus.GetReason() !=
		mesos_v1.TaskStatus_REASON_RECONCILIATION {
		switch event.state {
		case pb_task.TaskState_RUNNING:
			p.metrics.TasksRunningTotal.Inc(1)
		case pb_task.TaskState_SUCCEEDED:
			p.metrics.TasksSucceededTotal.Inc(1)
		case pb_task.TaskState_FAILED:
			p.metrics.TasksFailedTotal.Inc(1)
			p.metrics.TasksFailedReason[int32(event.mesosTaskStatus.GetReason())].Inc(1)
			log.WithFields(log.Fields{
				"task_id":       event.taskID,
				"failed_reason": mesos_v1.TaskStatus_Reason_name[int32(event.mesosTaskStatus.GetReason())],
			}).Debug("received failed task")
		case pb_task.TaskState_KILLED:
			p.metrics.TasksKilledTotal.Inc(1)
		case pb_task.TaskState_LOST:
			p.metrics.TasksLostTotal.Inc(1)
		case pb_task.TaskState_LAUNCHED:
			p.metrics.TasksLaunchedTotal.Inc(1)
		case pb_task.TaskState_STARTING:
			p.metrics.TasksStartingTotal.Inc(1)
		}
	} else if event.isMesosStatus && event.mesosTaskStatus.GetReason() ==
		mesos_v1.TaskStatus_REASON_RECONCILIATION {
		p.metrics.TasksReconciledTotal.Inc(1)
	}
}

// isOrphanTaskEvent returns if a task event is from orphan task,
// it returns the TaskInfo if task is not orphan
func (p *statusUpdate) isOrphanTaskEvent(
	ctx context.Context,
	event *statusUpateEvent,
) (bool, *pb_task.TaskInfo, error) {
	taskInfo, err := p.taskStore.GetTaskByID(ctx, event.taskID)
	if err != nil {
		if yarpcerrors.IsNotFound(err) {
			// if task runtime or config is not present in the DB,
			// then the task is orphan
			log.WithFields(log.Fields{
				"mesos_task_id":      event.mesosTaskStatus,
				"task_status_event≠": event.state.String(),
			}).Info("received status update for task not found in DB")
			return true, nil, nil
		}

		log.WithError(err).
			WithField("task_id", event.taskID).
			WithField("task_status_event", event.mesosTaskStatus).
			WithField("state", event.state.String()).
			Error("fail to find taskInfo for taskID for mesos event")
		return false, nil, err
	}

	dbTaskID := taskInfo.GetRuntime().GetMesosTaskId().GetValue()
	if event.isMesosStatus && dbTaskID !=
		event.mesosTaskStatus.GetTaskId().GetValue() {
		log.WithFields(log.Fields{
			"orphan_task_id":        event.mesosTaskStatus.GetTaskId().GetValue(),
			"db_task_id":            dbTaskID,
			"db_task_runtime_state": taskInfo.GetRuntime().GetState().String(),
			"mesos_event_state":     event.state.String(),
		}).Info("received status update for orphan mesos task")
		return true, nil, nil
	}

	return false, taskInfo, nil
}

// updatePersistentVolumeState updates volume state to be CREATED.
func (p *statusUpdate) updatePersistentVolumeState(ctx context.Context, taskInfo *pb_task.TaskInfo) error {
	// Update volume state to be created if task enters RUNNING state.
	volumeInfo, err := p.volumeStore.GetPersistentVolume(ctx, taskInfo.GetRuntime().GetVolumeID())
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"job_id":          taskInfo.GetJobId().GetValue(),
			"instance_id":     taskInfo.GetInstanceId(),
			"db_task_runtime": taskInfo.GetRuntime(),
			"volume_id":       taskInfo.GetRuntime().GetVolumeID(),
		}).Error("Failed to read db for given volume")
		_, ok := err.(*storage.VolumeNotFoundError)
		if !ok {
			// Do not ack status update running if db read error.
			return err
		}
		return nil
	}

	// Do not update volume db if state is already CREATED or goalstate is DELETED.
	if volumeInfo.GetState() == volume.VolumeState_CREATED ||
		volumeInfo.GetGoalState() == volume.VolumeState_DELETED {
		return nil
	}

	volumeInfo.State = volume.VolumeState_CREATED
	return p.volumeStore.UpdatePersistentVolume(ctx, volumeInfo)
}

func (p *statusUpdate) ProcessListeners(event *pb_eventstream.Event) {
	for _, listener := range p.listeners {
		listener.OnEvents([]*pb_eventstream.Event{event})
	}
}

// OnEvents is the callback function notifying a batch of events
func (p *statusUpdate) OnEvents(events []*pb_eventstream.Event) {}

// Start starts processing status update events
func (p *statusUpdate) Start() {
	p.applier.start()
	for _, client := range p.eventClients {
		client.Start()
	}
	log.Info("Task status updater started")
	for _, listener := range p.listeners {
		listener.Start()
	}
}

// Stop stops processing status update events
func (p *statusUpdate) Stop() {
	for _, client := range p.eventClients {
		client.Stop()
	}
	log.Info("Task status updater stopped")
	for _, listener := range p.listeners {
		listener.Stop()
	}
	p.applier.drainAndShutdown()
}

func getCurrTaskResourceUsage(taskID string, state pb_task.TaskState,
	resourceCfg *pb_task.ResourceConfig,
	startTime, completionTime string) map[string]float64 {
	currTaskResourceUsage, err := jobmgr_task.CreateResourceUsageMap(
		resourceCfg, startTime, completionTime)
	if err != nil {
		// only log the error here and continue processing the event
		// in this case resource usage map will be nil
		log.WithError(err).
			WithFields(log.Fields{
				"task_id": taskID,
				"state":   state}).
			Error("failed to calculate resource usage")
	}
	return currTaskResourceUsage
}

// persistHealthyField update the healthy field in runtimeDiff
func (p *statusUpdate) persistHealthyField(
	state pb_task.TaskState,
	reason mesos_v1.TaskStatus_Reason,
	healthy bool,
	newRuntime *pb_task.RuntimeInfo) {

	switch {
	case util.IsPelotonStateTerminal(state):
		// Set healthy to INVALID for all terminal state
		newRuntime.Healthy = pb_task.HealthState_INVALID
	case state == pb_task.TaskState_RUNNING:
		// Only record the health check result when
		// the reason for the event is TASK_HEALTH_CHECK_STATUS_UPDATED
		if reason == mesos_v1.TaskStatus_REASON_TASK_HEALTH_CHECK_STATUS_UPDATED {
			newRuntime.Reason = reason.String()
			if healthy {
				newRuntime.Healthy = pb_task.HealthState_HEALTHY
				p.metrics.TasksHealthyTotal.Inc(1)
			} else {
				newRuntime.Healthy = pb_task.HealthState_UNHEALTHY
				p.metrics.TasksUnHealthyTotal.Inc(1)
			}
		}
	}
}

func updateFailureCount(
	eventState pb_task.TaskState,
	runtime *pb_task.RuntimeInfo,
	newRuntime *pb_task.RuntimeInfo) {

	if !util.IsPelotonStateTerminal(eventState) {
		return
	}

	if runtime.GetConfigVersion() != runtime.GetDesiredConfigVersion() {
		// do not increment the failure count if config version has changed
		return
	}

	switch {

	case eventState == pb_task.TaskState_FAILED:
		newRuntime.FailureCount = runtime.GetFailureCount() + 1

	case eventState == pb_task.TaskState_SUCCEEDED &&
		runtime.GetGoalState() == pb_task.TaskState_RUNNING:
		newRuntime.FailureCount = runtime.GetFailureCount() + 1

	case eventState == pb_task.TaskState_KILLED &&
		runtime.GetGoalState() != pb_task.TaskState_KILLED:
		// This KILLED event is unexpected
		newRuntime.FailureCount = runtime.GetFailureCount() + 1
	}
}

// isDuplicateStateUpdate validates if the current instance state is left unchanged
// by this status update.
// If it is left unchanged, then the status update should be ignored.
// The state is said to be left unchanged
// if any of the following conditions is satisfied.
//
// 1. State is the same and that state is not running.
// 2. State is the same, that state is running, and health check is not configured.
// 3. State is the same, that state is running, and the update is not due to health check result.
// 4. State is the same, that state is running, the update is due to health check result and the task is healthy.
//
// Each unhealthy state needs to be logged into the pod events table.
func isDuplicateStateUpdate(
	taskInfo *pb_task.TaskInfo,
	event *pb_eventstream.Event,
	newState pb_task.TaskState) bool {

	if newState != taskInfo.GetRuntime().GetState() {
		return false
	}

	if newState != pb_task.TaskState_RUNNING {
		log.WithFields(log.Fields{
			"db_task_runtime":   taskInfo.GetRuntime(),
			"task_status_event": event.GetMesosTaskStatus(),
		}).Debug("skip same status update if state is not RUNNING")
		return true
	}

	if taskInfo.GetConfig().GetHealthCheck() == nil ||
		!taskInfo.GetConfig().GetHealthCheck().GetEnabled() {
		log.WithFields(log.Fields{
			"db_task_runtime":   taskInfo.GetRuntime(),
			"task_status_event": event.GetMesosTaskStatus(),
		}).Debug("skip same status update if health check is not configured or " +
			"disabled")
		return true
	}

	newStateReason := event.GetMesosTaskStatus().GetReason()
	if newStateReason != mesos_v1.TaskStatus_REASON_TASK_HEALTH_CHECK_STATUS_UPDATED {
		log.WithFields(log.Fields{
			"db_task_runtime":   taskInfo.GetRuntime(),
			"task_status_event": event.GetMesosTaskStatus(),
		}).Debug("skip same status update if status update reason is not from health check")
		return true
	}

	// Current behavior will log consecutive negative health check results
	// ToDo (varung): Evaluate if consecutive negative results should be logged or not
	isPreviousStateHealthy := taskInfo.GetRuntime().GetHealthy() == pb_task.HealthState_HEALTHY
	if !isPreviousStateHealthy {
		log.WithFields(log.Fields{
			"db_task_runtime":   taskInfo.GetRuntime(),
			"task_status_event": event.GetMesosTaskStatus(),
		}).Debug("log each negative health check result")
		return false
	}

	if event.GetMesosTaskStatus().GetHealthy() == isPreviousStateHealthy {
		log.WithFields(log.Fields{
			"db_task_runtime":   taskInfo.GetRuntime(),
			"task_status_event": event.GetMesosTaskStatus(),
		}).Debug("skip same status update if health check result is positive consecutively")
		return true
	}

	return false
}
