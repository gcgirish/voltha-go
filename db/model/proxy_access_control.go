/*
 * Copyright 2018-present Open Networking Foundation

 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at

 * http://www.apache.org/licenses/LICENSE-2.0

 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package model

import (
	"github.com/opencord/voltha-go/common/log"
	"runtime/debug"
	"sync"
	"time"
)

type singletonProxyAccessControl struct {
	sync.RWMutex
	cache map[string]ProxyAccessControl
}

var instanceProxyAccessControl *singletonProxyAccessControl
var onceProxyAccessControl sync.Once

// PAC provides access to the proxy access control singleton instance
func PAC() *singletonProxyAccessControl {
	onceProxyAccessControl.Do(func() {
		instanceProxyAccessControl = &singletonProxyAccessControl{cache: make(map[string]ProxyAccessControl)}
	})
	return instanceProxyAccessControl
}

// ReservePath will apply access control for a specific path within the model
func (singleton *singletonProxyAccessControl) ReservePath(path string, proxy *Proxy, pathLock string) ProxyAccessControl {
	singleton.Lock()
	defer singleton.Unlock()
	var pac ProxyAccessControl
	var exists bool
	if pac, exists = singleton.cache[path]; !exists {
		pac = NewProxyAccessControl(proxy, pathLock)
		singleton.cache[path] = pac
	}

	if exists {
		log.Debugf("PAC exists for path: %s... re-using", path)
	} else {
		log.Debugf("PAC does not exists for path: %s... creating", path)
	}
	return pac
}

// ReleasePath will remove access control for a specific path within the model
func (singleton *singletonProxyAccessControl) ReleasePath(pathLock string) {
	singleton.Lock()
	defer singleton.Unlock()
	delete(singleton.cache, pathLock)
}

// ProxyAccessControl is the abstraction interface to the base proxyAccessControl structure
type ProxyAccessControl interface {
	Get(path string, depth int, deep bool, txid string, control bool) interface{}
	Update(path string, data interface{}, strict bool, txid string, control bool) interface{}
	Add(path string, data interface{}, txid string, control bool) interface{}
	Remove(path string, txid string, control bool) interface{}
	SetProxy(proxy *Proxy)
}

// proxyAccessControl holds details of the path and proxy that requires access control
type proxyAccessControl struct {
	sync.RWMutex
	Proxy    *Proxy
	PathLock chan struct{}
	Path     string

	start time.Time
	stop  time.Time
}

// NewProxyAccessControl creates a new instance of an access control structure
func NewProxyAccessControl(proxy *Proxy, path string) ProxyAccessControl {
	return &proxyAccessControl{
		Proxy:    proxy,
		Path:     path,
		PathLock: make(chan struct{}, 1),
	}
}

// lock will prevent access to a model path
func (pac *proxyAccessControl) lock() {
	pac.PathLock <- struct{}{}
	pac.setStart(time.Now())
}

// unlock will release control of a model path
func (pac *proxyAccessControl) unlock() {
	<-pac.PathLock
	pac.setStop(time.Now())
	GetProfiling().AddToInMemoryLockTime(pac.getStop().Sub(pac.getStart()).Seconds())
}

// getStart is used for profiling purposes and returns the time at which access control was applied
func (pac *proxyAccessControl) getStart() time.Time {
	pac.Lock()
	defer pac.Unlock()
	return pac.start
}

// getStart is used for profiling purposes and returns the time at which access control was removed
func (pac *proxyAccessControl) getStop() time.Time {
	pac.Lock()
	defer pac.Unlock()
	return pac.stop
}

// getPath returns the access controlled path
func (pac *proxyAccessControl) getPath() string {
	pac.Lock()
	defer pac.Unlock()
	return pac.Path
}

// getProxy returns the proxy used to reach a specific location in the data model
func (pac *proxyAccessControl) getProxy() *Proxy {
	pac.Lock()
	defer pac.Unlock()
	return pac.Proxy
}

// setStart is for profiling purposes and applies a start time value at which access control was started
func (pac *proxyAccessControl) setStart(time time.Time) {
	pac.Lock()
	defer pac.Unlock()
	pac.start = time
}

// setStop is for profiling purposes and applies a stop time value at which access control was stopped
func (pac *proxyAccessControl) setStop(time time.Time) {
	pac.Lock()
	defer pac.Unlock()
	pac.stop = time
}

// SetProxy is used to changed the proxy object of an access controlled path
func (pac *proxyAccessControl) SetProxy(proxy *Proxy) {
	pac.Lock()
	defer pac.Unlock()
	pac.Proxy = proxy
}

// Get retrieves data linked to a data model path
func (pac *proxyAccessControl) Get(path string, depth int, deep bool, txid string, control bool) interface{} {
	if control {
		pac.lock()
		defer pac.unlock()
		log.Debugf("controlling get, stack = %s", string(debug.Stack()))
	}

	// FIXME: Forcing depth to 0 for now due to problems deep copying the data structure
	// The data traversal through reflection currently corrupts the content

	return pac.getProxy().GetRoot().Get(path, "", depth, deep, txid)
}

// Update changes the content of the data model at the specified location with the provided data
func (pac *proxyAccessControl) Update(path string, data interface{}, strict bool, txid string, control bool) interface{} {
	if control {
		pac.lock()
		defer pac.unlock()
		log.Debugf("controlling update, stack = %s", string(debug.Stack()))
	}
	result := pac.getProxy().GetRoot().Update(path, data, strict, txid, nil)

	if result != nil {
		return result.GetData()
	}
	return nil
}

// Add creates a new data model entry at the specified location with the provided data
func (pac *proxyAccessControl) Add(path string, data interface{}, txid string, control bool) interface{} {
	if control {
		pac.lock()
		defer pac.unlock()
		log.Debugf("controlling add, stack = %s", string(debug.Stack()))
	}
	result := pac.getProxy().GetRoot().Add(path, data, txid, nil)

	if result != nil {
		return result.GetData()
	}
	return nil
}

// Remove discards information linked to the data model path
func (pac *proxyAccessControl) Remove(path string, txid string, control bool) interface{} {
	if control {
		pac.lock()
		defer pac.unlock()
		log.Debugf("controlling remove, stack = %s", string(debug.Stack()))
	}
	return pac.getProxy().GetRoot().Remove(path, txid, nil)
}
