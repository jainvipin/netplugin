/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dockplugin

import (
	"encoding/json"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/authorization"
)

// authReq - request to allow/deny a container
func authReq(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		req     = authorization.Request{}
		decoder = json.NewDecoder(r.Body)
	)

	logEvent("authRequest")

	// Decode the JSON message
	err = decoder.Decode(&req)
	if err != nil {
		httpError(w, "Could not read and parse requestAddress request", err)
		return
	}

	log.Infof("Received AuthReq: %+v", req)

	// build response
	resp := authorization.Response{}
	resp.Allow = true
	resp.Msg = "you must be lucky..."

	log.Infof("Sending RequestAddressResponse: %+v", resp)

	// build json
	content, err = json.Marshal(resp)
	if err != nil {
		httpError(w, "Could not generate requestAddress response", err)
		return
	}

	w.Write(content)
}

// authResp - auth request after docker engine has processed the request
//  but before docker client is responded with
func authResp(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		req     = authorization.Request{}
		decoder = json.NewDecoder(r.Body)
	)

	logEvent("authResponse")

	// Decode the JSON message
	err = decoder.Decode(&req)
	if err != nil {
		httpError(w, "Could not read and parse requestAddress request", err)
		return
	}

	log.Infof("Received AuthReq: %+v", req)

	// build response
	resp := authorization.Response{}
	resp.Allow = true
	resp.Msg = "you must be lucky..."

	log.Infof("Sending RequestAddressResponse: %+v", resp)

	// build json
	content, err = json.Marshal(resp)
	if err != nil {
		httpError(w, "Could not generate requestAddress response", err)
		return
	}

	w.Write(content)
}
