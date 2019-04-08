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

package ssh

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClient_OpenFile(t *testing.T) {
	t.Skip()
	user := "vagrant"
	addr := "0.tcp.ngrok.io:11111"
	password := "-"

	c, err := NewClient(user, addr, password, nil)
	require.NoError(t, err)
	require.NotNil(t, c)

	f, err := c.Open("slurm-8.out")
	require.NoError(t, err)
	require.NotNil(t, f)
	defer f.Close()

	text, err := ioutil.ReadAll(f)
	require.NoError(t, err)
	require.NotZero(t, text)
	t.Log(string(text))
}
