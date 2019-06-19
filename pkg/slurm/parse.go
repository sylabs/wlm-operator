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

package slurm

import (
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	unlimited = "UNLIMITED"

	maxTime        = "MaxTime"
	maxNodes       = "MaxNodes"
	totalNodes     = "TotalNodes"
	maxCPUsPerNode = "MaxCPUsPerNode"
	totalCPUs      = "TotalCPUs"
	maxMemPerNode  = "MaxMemPerNode"
)

// ParseDuration parses slurm duration string. Possible formats are:
// minutes, minutes:seconds, hours:minutes:seconds, days-hours, days-hours:minutes or days-hours:minutes:seconds
func ParseDuration(duration string) (*time.Duration, error) {
	if duration == unlimited || duration == "" {
		return nil, ErrDurationIsUnlimited
	}

	var err error
	var d time.Duration
	var days, hours, minutes, seconds int64
	parts := strings.Split(duration, ":")
	if len(parts) > 3 {
		return nil, errors.New("invalid duration format")
	}
	i := strings.IndexByte(parts[0], '-')
	if i != -1 {
		days, err = strconv.ParseInt(parts[0][:i], 10, 0)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid amount of days")
		}
		hours, err = strconv.ParseInt(parts[0][i+1:], 10, 0)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid amount of hours")
		}
		if len(parts) > 1 {
			minutes, err = strconv.ParseInt(parts[1], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of minutes")
			}
		}
		if len(parts) > 2 {
			seconds, err = strconv.ParseInt(parts[2], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of seconds")
			}
		}
	} else {
		switch len(parts) {
		case 1:
			minutes, err = strconv.ParseInt(parts[0], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of minutes")
			}
		case 2:
			minutes, err = strconv.ParseInt(parts[0], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of minutes")
			}
			seconds, err = strconv.ParseInt(parts[1], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of seconds")
			}
		case 3:
			hours, err = strconv.ParseInt(parts[0], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of hours")
			}
			minutes, err = strconv.ParseInt(parts[1], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of minutes")
			}
			seconds, err = strconv.ParseInt(parts[2], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid amount of seconds")
			}
		}
	}

	d += time.Hour * 24 * time.Duration(days)
	d += time.Hour * time.Duration(hours)
	d += time.Minute * time.Duration(minutes)
	d += time.Second * time.Duration(seconds)
	return &d, nil
}

// parseResources parses scontrol output for a particular partition
// to fetch available resources.
func parseResources(partitionInfo string) (*Resources, error) {
	partitionInfo = strings.TrimSpace(partitionInfo)
	fields := strings.Fields(partitionInfo)

	fMap := make(map[string][]string)
	for _, f := range fields {
		s := strings.Split(f, "=")
		if len(s) != 2 {
			continue // skipping invalid or empty fields
		}
		fMap[s[0]] = append(fMap[s[0]], strings.Split(s[1], ",")...)
	}

	var resources Resources

	if maxTime, ok := fMap[maxTime]; ok {
		d, err := ParseDuration(maxTime[0])
		if err != nil {
			if err != ErrDurationIsUnlimited {
				return nil, errors.Wrap(err, "could not parse duration")
			}
			resources.WallTime = time.Duration(-1)
		} else {
			resources.WallTime = *d
		}
	}
	if maxCPUs, ok := fMap[maxCPUsPerNode]; ok {
		if maxCPUs[0] == unlimited {
			resources.CPUPerNode = -1
			totalCPUs, ok := fMap[totalCPUs]
			if ok {
				cpus, err := strconv.ParseInt(totalCPUs[0], 10, 0)
				if err != nil {
					return nil, errors.Wrap(err, "could not parse total cpus")
				}
				resources.CPUPerNode = cpus
			}
		} else {
			cpus, err := strconv.ParseInt(maxCPUs[0], 10, 0)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse max cpus num")
			}
			resources.CPUPerNode = cpus
		}
	}
	if maxMem, ok := fMap[maxMemPerNode]; ok {
		if maxMem[0] == unlimited {
			resources.MemPerNode = -1
		} else {
			mem, err := strconv.ParseInt(maxMem[0], 10, 0)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse max mem")
			}
			resources.MemPerNode = mem
		}
	}
	if maxNodes, ok := fMap[maxNodes]; ok {
		if maxNodes[0] == unlimited {
			resources.Nodes = -1
			vv, ok := fMap[totalNodes]
			if ok {
				nodes, err := strconv.ParseInt(vv[0], 10, 0)
				if err != nil {
					return nil, errors.Wrap(err, "could not parse total nodes")
				}
				resources.Nodes = nodes
			}
		} else {
			nodes, err := strconv.ParseInt(maxNodes[0], 10, 0)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse max nodes")
			}
			resources.Nodes = nodes
		}
	}

	return &resources, nil
}

// parsePartitionsNames extracts names from scontrol show partitions response.
func parsePartitionsNames(raw string) []string {
	const partitionNameF = "PartitionName"

	partitions := strings.Split(strings.TrimSpace(raw), "\n\n")
	names := make([]string, len(partitions))

	for i, p := range partitions {
		for _, f := range strings.Fields(p) {
			if s := strings.Split(f, "="); len(s) == 2 {
				if s[0] == partitionNameF {
					names[i] = s[1]
				}
			}
		}
	}

	return names
}

// parseSacctResponse is a helper that parses sacct output and
// returns results in a convenient form.
func parseSacctResponse(raw string) ([]*JobStepInfo, error) {
	lines := strings.Split(strings.Trim(raw, "\n"), "\n")
	infos := make([]*JobStepInfo, len(lines))
	for i, l := range lines {
		splitted := strings.Split(l, "|")
		if len(splitted) != 7 {
			return nil, errors.New("output must contain 6 sections")
		}

		startedAt, err := parseTime(splitted[0])
		if err != nil {
			return nil, err
		}

		finishedAt, err := parseTime(splitted[1])
		if err != nil {
			return nil, err
		}

		exitCodeSplitted := strings.Split(splitted[2], ":")
		if len(exitCodeSplitted) != 2 {
			return nil, errors.New("exit code must contain 2 sections")
		}
		exitCode, err := strconv.Atoi(exitCodeSplitted[0])
		if err != nil {
			return nil, err
		}
		j := JobStepInfo{
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			ExitCode:   exitCode,
			State:      splitted[3],
			ID:         splitted[4],
			Name:       splitted[5],
		}
		infos[i] = &j
	}

	return infos, nil
}

func parseTime(timeStr string) (*time.Time, error) {
	const slurmTimeLayout = "2006-01-02T15:04:05"

	if timeStr == "" || strings.ToLower(timeStr) == "unknown" {
		return nil, nil
	}

	t, err := time.Parse(slurmTimeLayout, timeStr)
	if err != nil {
		return nil, err
	}

	return &t, nil
}
