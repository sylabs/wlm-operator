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

package api

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/sylabs/slurm-operator/pkg/slurm/local"
)

func newTestServer(_ *testing.T) (*httptest.Server, func()) {
	slurmClient := &local.Client{}

	router := NewSlurmRouter(slurmClient)
	srv := httptest.NewServer(router)
	return srv, func() {
		srv.CloseClientConnections()
		srv.Close()
	}
}

func TestApi_Open(t *testing.T) {
	testFile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer os.Remove(testFile.Name())

	fileContent := []byte(`
Hello!
This is a test output from SLURM!
`)

	_, err = testFile.Write(fileContent)
	require.NoError(t, err)
	require.NoError(t, testFile.Close())

	srv, cleanup := newTestServer(t)
	defer cleanup()

	t.Run("no path", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/open", srv.URL), nil)
		require.NoError(t, err)
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		content, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "no path query parameter is found\n", string(content))
	})

	t.Run("non existent file", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/open?path=/foo/bar", srv.URL), nil)
		require.NoError(t, err)
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("all ok", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/open?path=%s", srv.URL, testFile.Name()), nil)
		require.NoError(t, err)
		resp, err := srv.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		content, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, fileContent, content, "unexpected file content")
	})
}

func TestApi_Tail(t *testing.T) {
	testFile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer os.Remove(testFile.Name())
	defer testFile.Close()
	writerCtx, wrCancel := context.WithCancel(context.Background())
	readerCtx, rCancel := context.WithCancel(context.Background())
	testT := time.NewTimer(10 * time.Second)
	go func() {
		<-testT.C
		wrCancel()
		// need give reader some time to read out all data
		<-time.NewTimer(2 * time.Second).C
		rCancel()
	}()

	tick := time.NewTicker(1 * time.Second)
	writeCount := 0
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				log.Println("Logs finished")
				return
			case <-tick.C:
				testFile.Write([]byte("test\n"))
				writeCount++
			}
		}
	}(writerCtx)

	srv, cleanup := newTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/tail?path=%s", srv.URL, testFile.Name()), nil)
	require.NoError(t, err)
	req = req.WithContext(readerCtx)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	require.EqualValues(t, 200, resp.StatusCode)

	b := &bytes.Buffer{}
	readCount := 0

	for {
		buff := make([]byte, 128)
		n, err := resp.Body.Read(buff)
		if err != nil {
			require.EqualValues(t, "context canceled", err.Error())
			break
		}
		b.Write(buff[:n])
		readCount++
	}

	require.EqualValues(t, writeCount, readCount)
	fi, err := testFile.Stat()
	require.NoError(t, err)
	require.EqualValues(t, fi.Size(), b.Len())
}
