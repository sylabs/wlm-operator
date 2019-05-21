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

	"github.com/pkg/errors"
	"github.com/sylabs/slurm-operator/pkg/slurm"
)

// extractBatchResources extracts resources that should be satisfied for a slurm
// job to run. More particularly, the following SBATCH directives are parsed:
// nodes, time, mem, ntasks and/or (n)tasks-per-node.
// A zero value is returned if corresponding value is not provided.
func extractBatchResources(script string) (*slurm.Resources, error) {
	const (
		sbatchHeader   = "#SBATCH"
		timeLimit      = "--time"
		timeLimitShort = "-t"
		nodes          = "--nodes"
		nodesShort     = "-N"
	)

	var res slurm.Resources
	s := bufio.NewScanner(strings.NewReader(script))
	for s.Scan() {
		line := s.Text()
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

			switch param {
			case timeLimit, timeLimitShort:
				duration, err := slurm.ParseDuration(value)
				if err != nil {
					return nil, errors.Wrapf(err, "could not parse time limit")
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
					return nil, errors.Wrapf(err, "could not parse amount of nodes")
				}
				res.Nodes = nodes
			}
		}
	}
	return &res, nil
}
