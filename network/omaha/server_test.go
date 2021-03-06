// Copyright 2015 CoreOS, Inc.
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

package omaha

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

type mockServer struct {
	UpdaterStub

	reqChan chan *Request
}

func (m *mockServer) CheckApp(req *Request, app *AppRequest) error {
	m.reqChan <- req
	return nil
}

func TestServerRequestResponse(t *testing.T) {
	var wg sync.WaitGroup
	defer wg.Wait()

	// make an omaha server
	svc := &mockServer{
		reqChan: make(chan *Request),
	}

	s, err := NewServer(":0", svc)
	if err != nil {
		t.Fatalf("failed to create omaha server: %v", err)
	}
	defer func() {
		err := s.Destroy()
		if err != nil {
			panic(err)
		}
		close(svc.reqChan)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Serve(); err != nil {
			t.Errorf("Serve failed: %v", err)
		}
	}()

	buf := new(bytes.Buffer)
	enc := xml.NewEncoder(buf)
	enc.Indent("", "\t")
	err = enc.Encode(nilRequest)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// check that server gets the same thing we sent
	wg.Add(1)
	go func() {
		defer wg.Done()
		sreq, ok := <-svc.reqChan
		if !ok {
			t.Errorf("failed to get notification from server")
			return
		}

		if err := compareXML(nilRequest, sreq); err != nil {
			t.Error(err)
		}
	}()

	// send omaha request
	endpoint := fmt.Sprintf("http://%s/v1/update/", s.Addr())
	httpClient := &http.Client{
		Timeout: 2 * time.Second,
	}
	res, err := httpClient.Post(endpoint, "text/xml", buf)
	if err != nil {
		t.Fatalf("failed to post: %v", err)
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		t.Fatalf("failed to post: %v", res.Status)
	}

	dec := xml.NewDecoder(res.Body)
	sresp := &Response{}
	if err := dec.Decode(sresp); err != nil {
		t.Fatalf("failed to parse body: %v", err)
	}
	if err := compareXML(nilResponse, sresp); err != nil {
		t.Error(err)
	}
}
