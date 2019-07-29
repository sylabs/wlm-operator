// Copyright (c) 2019 Sylabs, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slurmjob

import (
	"bufio"
	"strconv"
	"strings"

	"github.com/sylabs/wlm-operator/pkg/operator/controller"

	"github.com/pkg/errors"
	"github.com/sylabs/wlm-operator/pkg/slurm"
)

// extractBatchResources extracts resources that should be satisfied for a slurm
// job to run. More particularly, the following SBATCH directives are parsed:
// nodes, time, mem, ntasks and/or (n)tasks-per-node.
// A zero value is returned if corresponding value is not provided.
func extractBatchResources(script string) (*controller.Resources, error) {
	const sbatchHeader = "#SBATCH"

	var err error
	var res controller.Resources

	s := bufio.NewScanner(strings.NewReader(script))
	for s.Scan() {
		line := s.Text()
		// skip empty lines and shebang
		if line == "" || strings.HasPrefix(line, "#!") {
			continue
		}
		// #SBATCH headers go first, so stop whenever they are finished
		if !strings.HasPrefix(line, sbatchHeader) {
			break
		}

		params := strings.Fields(strings.TrimPrefix(line, sbatchHeader))
		for j := 0; j < len(params); j++ {
			param := params[j]
			var value string
			// handle both '--param=value' and
			// '--param value' cases
			i := strings.IndexByte(param, '=')
			if i != -1 {
				value = param[i+1:]
				param = param[:i]
			} else if i < len(params)-1 {
				value = params[j+1]
				j++
			}
			res, err = applySbatchParam(res, param, value)
			if err != nil {
				return nil, err
			}
		}
	}
	return &res, nil
}

func applySbatchParam(res controller.Resources, param, value string) (controller.Resources, error) {
	const (
		timeLimit        = "--time"
		timeLimitShort   = "-t"
		nodes            = "--nodes"
		nodesShort       = "-N"
		mem              = "--mem"
		tasksPerNode     = "--ntasks-per-node"
		cpusPerTask      = "--cpus-per-task"
		cpusPerTaskShort = "-c"
	)

	switch param {
	case timeLimit, timeLimitShort:
		duration, err := slurm.ParseDuration(value)
		if err != nil && err != slurm.ErrDurationIsUnlimited {
			return controller.Resources{}, errors.Wrapf(err, "could not parse time limit")
		}
		if duration != nil {
			res.WallTime = *duration
		}
	case nodes, nodesShort:
		i := strings.IndexByte(value, '-')
		// we use min nodes value only
		if i != -1 {
			value = value[:i]
		}
		nodes, err := strconv.ParseInt(value, 10, 0)
		if err != nil {
			return controller.Resources{}, errors.Wrapf(err, "could not parse amount of nodes")
		}
		res.Nodes = nodes
	case mem:
		// suffixes are not supported yet
		mem, err := strconv.ParseInt(value, 10, 0)
		if err != nil {
			return controller.Resources{}, errors.Wrapf(err, "could not parse memory")
		}
		res.MemPerNode = mem
	case cpusPerTask, cpusPerTaskShort:
		cpus, err := strconv.ParseInt(value, 10, 0)
		if err != nil {
			return controller.Resources{}, errors.Wrapf(err, "could not parse cpus per node")
		}
		if res.CPUPerNode == 0 {
			res.CPUPerNode = 1
		}
		res.CPUPerNode *= cpus
	case tasksPerNode:
		tasks, err := strconv.ParseInt(value, 10, 0)
		if err != nil {
			return controller.Resources{}, errors.Wrapf(err, "could not parse tasks per node")
		}
		if res.CPUPerNode == 0 {
			res.CPUPerNode = 1
		}
		res.CPUPerNode *= tasks
	}
	return res, nil
}
