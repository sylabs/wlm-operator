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

package local

import (
	"bytes"
	"io"
	"log"
	"os"

	"github.com/sylabs/slurm-operator/pkg/slurm"

	"github.com/hpcloud/tail"
)

type TailReader struct {
	t *tail.Tail

	buff *bytes.Buffer

	isClosed bool
}

func NewTailReader(path string) (*TailReader, error) {
	t, err := tail.TailFile(path, tail.Config{Follow: true, MustExist: true})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, slurm.ErrFileNotFound
		}

		return nil, err
	}

	tr := &TailReader{
		t:    t,
		buff: &bytes.Buffer{},
	}

	go tr.readTail()

	return tr, nil
}

// Read returns EOF error only after invoking Close.
// Before close in case of EOF errors it will be returning nil
func (tr *TailReader) Read(p []byte) (int, error) {
	n, err := io.ReadFull(tr.buff, p)
	if err == io.EOF || err == io.ErrUnexpectedEOF && !tr.isClosed {
		err = nil
	}

	return n, err
}

func (tr *TailReader) Close() error {
	_ = tr.t.StopAtEOF() // it returns stop reason instead of err
	return nil
}

func (tr *TailReader) readTail() {
	defer func() {
		log.Println("Read tail finished")
		tr.isClosed = true
	}()

	for {
		l, ok := <-tr.t.Lines
		if !ok {
			return
		}

		if l.Err != nil {
			log.Printf("Tail line err: %s", l.Err)
			return
		}

		_, err := tr.buff.Write([]byte(l.Text + "\n"))
		if err != nil {
			log.Printf("Could not write to buffer err: %s", err)
			return
		}
	}
}
