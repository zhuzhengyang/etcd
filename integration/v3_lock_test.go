// Copyright 2016 CoreOS, Inc.
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
package integration

import (
	"math/rand"
	"testing"
	"time"

	"github.com/zhuzhengyang/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/zhuzhengyang/etcd/clientv3"
	"github.com/zhuzhengyang/etcd/clientv3/concurrency"
	"github.com/zhuzhengyang/etcd/contrib/recipes"
)

func TestMutexSingleNode(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)
	testMutex(t, 5, func() *clientv3.Client { return clus.clients[0] })
}

func TestMutexMultiNode(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)
	testMutex(t, 5, func() *clientv3.Client { return clus.RandClient() })
}

func testMutex(t *testing.T, waiters int, chooseClient func() *clientv3.Client) {
	// stream lock acquisitions
	lockedC := make(chan *concurrency.Mutex, 1)
	for i := 0; i < waiters; i++ {
		go func() {
			m := concurrency.NewMutex(chooseClient(), "test-mutex")
			if err := m.Lock(context.TODO()); err != nil {
				t.Fatalf("could not wait on lock (%v)", err)
			}
			lockedC <- m
		}()
	}
	// unlock locked mutexes
	timerC := time.After(time.Duration(waiters) * time.Second)
	for i := 0; i < waiters; i++ {
		select {
		case <-timerC:
			t.Fatalf("timed out waiting for lock %d", i)
		case m := <-lockedC:
			// lock acquired with m
			select {
			case <-lockedC:
				t.Fatalf("lock %d followers did not wait", i)
			default:
			}
			if err := m.Unlock(); err != nil {
				t.Fatalf("could not release lock (%v)", err)
			}
		}
	}
}

func BenchmarkMutex4Waiters(b *testing.B) {
	// XXX switch tests to use TB interface
	clus := NewClusterV3(nil, &ClusterConfig{Size: 3})
	defer clus.Terminate(nil)
	for i := 0; i < b.N; i++ {
		testMutex(nil, 4, func() *clientv3.Client { return clus.RandClient() })
	}
}

func TestRWMutexSingleNode(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)
	testRWMutex(t, 5, func() *clientv3.Client { return clus.clients[0] })
}

func TestRWMutexMultiNode(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)
	testRWMutex(t, 5, func() *clientv3.Client { return clus.RandClient() })
}

func testRWMutex(t *testing.T, waiters int, chooseClient func() *clientv3.Client) {
	// stream rwlock acquistions
	rlockedC := make(chan *recipe.RWMutex, 1)
	wlockedC := make(chan *recipe.RWMutex, 1)
	for i := 0; i < waiters; i++ {
		go func() {
			rwm := recipe.NewRWMutex(chooseClient(), "test-rwmutex")
			if rand.Intn(1) == 0 {
				if err := rwm.RLock(); err != nil {
					t.Fatalf("could not rlock (%v)", err)
				}
				rlockedC <- rwm
			} else {
				if err := rwm.Lock(); err != nil {
					t.Fatalf("could not lock (%v)", err)
				}
				wlockedC <- rwm
			}
		}()
	}
	// unlock locked rwmutexes
	timerC := time.After(time.Duration(waiters) * time.Second)
	for i := 0; i < waiters; i++ {
		select {
		case <-timerC:
			t.Fatalf("timed out waiting for lock %d", i)
		case wl := <-wlockedC:
			select {
			case <-rlockedC:
				t.Fatalf("rlock %d readers did not wait", i)
			default:
			}
			if err := wl.Unlock(); err != nil {
				t.Fatalf("could not release lock (%v)", err)
			}
		case rl := <-rlockedC:
			select {
			case <-wlockedC:
				t.Fatalf("rlock %d writers did not wait", i)
			default:
			}
			if err := rl.RUnlock(); err != nil {
				t.Fatalf("could not release rlock (%v)", err)
			}
		}
	}
}
