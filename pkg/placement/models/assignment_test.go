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

package models

import (
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/peloton/.gen/peloton/api/v0/job"
	peloton_api_v0_task "github.com/uber/peloton/.gen/peloton/api/v0/task"
	"github.com/uber/peloton/.gen/peloton/private/hostmgr/hostsvc"
	"github.com/uber/peloton/.gen/peloton/private/resmgr"
	"github.com/uber/peloton/.gen/peloton/private/resmgrsvc"

	"github.com/uber/peloton/pkg/hostmgr/scalar"
)

func setupAssignmentVariables() (
	*hostsvc.HostOffer,
	*resmgrsvc.Gang,
	*resmgr.Task,
	*HostOffers,
	*Task,
	*Assignment) {
	resmgrTask := &resmgr.Task{
		Name: "task",
		Resource: &peloton_api_v0_task.ResourceConfig{
			CpuLimit:   1.0,
			MemLimitMb: 1.0,
		},
		NumPorts: 10,
	}
	hostOffer := &hostsvc.HostOffer{
		Hostname: "hostname",
	}
	now := time.Now()
	offer := NewHostOffers(hostOffer, []*resmgr.Task{resmgrTask}, now)
	resmgrGang := &resmgrsvc.Gang{
		Tasks: []*resmgr.Task{
			resmgrTask,
		},
	}
	task := NewTask(resmgrGang, resmgrTask, now.Add(5*time.Second), now, 3)
	assignment := NewAssignment(task)
	return hostOffer, resmgrGang, resmgrTask, offer, task, assignment
}

func TestAssignment(t *testing.T) {
	t.Run("task", func(t *testing.T) {
		_, _, _, _, task, assignment := setupAssignmentVariables()
		assert.Equal(t, task, assignment.GetTask())

		task.SetMaxRounds(5)
		assignment.SetTask(task)
		assert.Equal(t, 5, assignment.GetTask().GetMaxRounds())
	})

	t.Run("offer", func(t *testing.T) {
		_, _, _, _, _, assignment := setupAssignmentVariables()
		assert.Nil(t, assignment.GetHost())
	})

	t.Run("set offer", func(t *testing.T) {
		_, _, _, host, _, assignment := setupAssignmentVariables()
		assignment.SetHost(host)
		assert.Equal(t, host, assignment.GetHost())
	})

	t.Run("log fields", func(t *testing.T) {
		log.SetFormatter(&log.JSONFormatter{})
		initialLevel := log.DebugLevel
		log.SetLevel(initialLevel)

		_, _, _, host, _, assignment := setupAssignmentVariables()
		assignment.SetHost(host)
		entry, err := log.WithField("foo", assignment).String()
		assert.NoError(t, err)
		assert.Contains(t, entry, "foo")
		assert.Contains(t, entry, "host")
		assert.Contains(t, entry, "offer")
		assert.Contains(t, entry, "tasks")
		assert.Contains(t, entry, "claimed")
		assert.Contains(t, entry, "deadline")
		assert.Contains(t, entry, "max_rounds")
		assert.Contains(t, entry, "rounds")
	})

	t.Run("nil constraint", func(t *testing.T) {
		_, _, _, _, _, assignment := setupAssignmentVariables()
		constraint := assignment.GetConstraint()
		require.Nil(t, constraint)
	})

	t.Run("host filter", func(t *testing.T) {
		_, _, _, _, _, assignment := setupAssignmentVariables()
		assignment.GetTask().GetTask().PlacementStrategy = job.PlacementStrategy_PLACEMENT_STRATEGY_SPREAD_JOB
		simpleFilter := assignment.GetSimpleHostFilter()
		require.Nil(t, simpleFilter.SchedulingConstraint)
		require.Equal(t, hostsvc.FilterHint_FILTER_HINT_RANKING_RANDOM, simpleFilter.Hint.RankHint)

		_, _, _, _, _, assignment = setupAssignmentVariables()
		assignment.GetTask().GetTask().PlacementStrategy = job.PlacementStrategy_PLACEMENT_STRATEGY_SPREAD_JOB
		fullFilter := assignment.GetFullHostFilter()
		require.Nil(t, fullFilter.SchedulingConstraint)
		require.Equal(t, hostsvc.FilterHint_FILTER_HINT_RANKING_RANDOM, fullFilter.Hint.RankHint)
		require.Equal(t, uint32(1), fullFilter.Quantity.MaxHosts)
	})

	t.Run("merge filter", func(t *testing.T) {
		_, _, _, _, _, a1 := setupAssignmentVariables()
		_, _, _, _, _, a2 := setupAssignmentVariables()
		assignments := Assignments([]*Assignment{a1, a2})
		filter := assignments.MergeHostFilters()
		require.Nil(t, filter.SchedulingConstraint)
		require.Equal(t, uint32(2), filter.Quantity.MaxHosts)
	})

	t.Run("fits", func(t *testing.T) {
		_, _, _, _, _, a1 := setupAssignmentVariables()
		resLeft := scalar.Resources{
			CPU: 1.0,
			Mem: 2.0,
		}
		portsLeft := uint64(20)
		resLeft, portsLeft, fit := a1.Fits(resLeft, portsLeft)
		require.True(t, fit)
		require.Equal(t, uint64(10), portsLeft)
		require.Equal(t, float64(0), resLeft.CPU)
		require.Equal(t, float64(1), resLeft.Mem)

		resLeft, portsLeft, fit = a1.Fits(resLeft, portsLeft)
		require.False(t, fit)
		require.Equal(t, uint64(10), portsLeft)
		require.Equal(t, float64(0), resLeft.CPU)
		require.Equal(t, float64(1), resLeft.Mem)

		portsLeft = 5
		resLeft, portsLeft, fit = a1.Fits(resLeft, portsLeft)
		require.False(t, fit)
		require.Equal(t, uint64(5), portsLeft)
		require.Equal(t, float64(0), resLeft.CPU)
		require.Equal(t, float64(1), resLeft.Mem)
	})
}
