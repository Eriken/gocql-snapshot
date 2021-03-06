// Copyright (c) 2015 The gocql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gocql

import (
	"fmt"
	"github.com/Eriken/gocql-snapshot/internal/streams"
	"testing"

	"github.com/hailocab/go-hostpool"
)

// Tests of the round-robin host selection policy implementation
func TestRoundRobinHostPolicy(t *testing.T) {
	policy := RoundRobinHostPolicy()

	hosts := []*HostInfo{
		{hostId: "0"},
		{hostId: "1"},
	}

	policy.SetHosts(hosts)

	// interleaved iteration should always increment the host
	iterA := policy.Pick(nil)
	if actual := iterA(); actual.Info() != hosts[0] {
		t.Errorf("Expected hosts[0] but was hosts[%s]", actual.Info().HostID())
	}
	iterB := policy.Pick(nil)
	if actual := iterB(); actual.Info() != hosts[1] {
		t.Errorf("Expected hosts[1] but was hosts[%s]", actual.Info().HostID())
	}
	if actual := iterB(); actual.Info() != hosts[0] {
		t.Errorf("Expected hosts[0] but was hosts[%s]", actual.Info().HostID())
	}
	if actual := iterA(); actual.Info() != hosts[1] {
		t.Errorf("Expected hosts[1] but was hosts[%s]", actual.Info().HostID())
	}

	iterC := policy.Pick(nil)
	if actual := iterC(); actual.Info() != hosts[0] {
		t.Errorf("Expected hosts[0] but was hosts[%s]", actual.Info().HostID())
	}
	if actual := iterC(); actual.Info() != hosts[1] {
		t.Errorf("Expected hosts[1] but was hosts[%s]", actual.Info().HostID())
	}
}

// Tests of the token-aware host selection policy implementation with a
// round-robin host selection policy fallback.
func TestTokenAwareHostPolicy(t *testing.T) {
	policy := TokenAwareHostPolicy(RoundRobinHostPolicy())

	query := &Query{}

	iter := policy.Pick(nil)
	if iter == nil {
		t.Fatal("host iterator was nil")
	}
	actual := iter()
	if actual != nil {
		t.Fatalf("expected nil from iterator, but was %v", actual)
	}

	// set the hosts
	hosts := []*HostInfo{
		{peer: "0", tokens: []string{"00"}},
		{peer: "1", tokens: []string{"25"}},
		{peer: "2", tokens: []string{"50"}},
		{peer: "3", tokens: []string{"75"}},
	}
	policy.SetHosts(hosts)

	// the token ring is not setup without the partitioner, but the fallback
	// should work
	if actual := policy.Pick(nil)(); actual.Info().Peer() != "0" {
		t.Errorf("Expected peer 0 but was %s", actual.Info().Peer())
	}

	query.RoutingKey([]byte("30"))
	if actual := policy.Pick(query)(); actual.Info().Peer() != "1" {
		t.Errorf("Expected peer 1 but was %s", actual.Info().Peer())
	}

	policy.SetPartitioner("OrderedPartitioner")

	// now the token ring is configured
	query.RoutingKey([]byte("20"))
	iter = policy.Pick(query)
	if actual := iter(); actual.Info().Peer() != "1" {
		t.Errorf("Expected peer 1 but was %s", actual.Info().Peer())
	}
	// rest are round robin
	if actual := iter(); actual.Info().Peer() != "2" {
		t.Errorf("Expected peer 2 but was %s", actual.Info().Peer())
	}
	if actual := iter(); actual.Info().Peer() != "3" {
		t.Errorf("Expected peer 3 but was %s", actual.Info().Peer())
	}
	if actual := iter(); actual.Info().Peer() != "0" {
		t.Errorf("Expected peer 0 but was %s", actual.Info().Peer())
	}
}

// Tests of the host pool host selection policy implementation
func TestHostPoolHostPolicy(t *testing.T) {
	policy := HostPoolHostPolicy(hostpool.New(nil))

	hosts := []*HostInfo{
		{hostId: "0", peer: "0"},
		{hostId: "1", peer: "1"},
	}

	policy.SetHosts(hosts)

	// the first host selected is actually at [1], but this is ok for RR
	// interleaved iteration should always increment the host
	iter := policy.Pick(nil)
	actualA := iter()
	if actualA.Info().HostID() != "0" {
		t.Errorf("Expected hosts[0] but was hosts[%s]", actualA.Info().HostID())
	}
	actualA.Mark(nil)

	actualB := iter()
	if actualB.Info().HostID() != "1" {
		t.Errorf("Expected hosts[1] but was hosts[%s]", actualB.Info().HostID())
	}
	actualB.Mark(fmt.Errorf("error"))

	actualC := iter()
	if actualC.Info().HostID() != "0" {
		t.Errorf("Expected hosts[0] but was hosts[%s]", actualC.Info().HostID())
	}
	actualC.Mark(nil)

	actualD := iter()
	if actualD.Info().HostID() != "0" {
		t.Errorf("Expected hosts[0] but was hosts[%s]", actualD.Info().HostID())
	}
	actualD.Mark(nil)
}

// Tests of the round-robin connection selection policy implementation
func TestRoundRobinConnPolicy(t *testing.T) {
	policy := RoundRobinConnPolicy()()

	conn0 := &Conn{streams: streams.New(1)}
	conn1 := &Conn{streams: streams.New(1)}
	conn := []*Conn{
		conn0,
		conn1,
	}

	policy.SetConns(conn)

	if actual := policy.Pick(nil); actual != conn0 {
		t.Error("Expected conn1")
	}
	if actual := policy.Pick(nil); actual != conn1 {
		t.Error("Expected conn0")
	}
	if actual := policy.Pick(nil); actual != conn0 {
		t.Error("Expected conn1")
	}
}

func TestRoundRobinNilHostInfo(t *testing.T) {
	policy := RoundRobinHostPolicy()

	host := &HostInfo{hostId: "host-1"}
	policy.SetHosts([]*HostInfo{host})

	iter := policy.Pick(nil)
	next := iter()
	if next == nil {
		t.Fatal("got nil host")
	} else if v := next.Info(); v == nil {
		t.Fatal("got nil HostInfo")
	} else if v.HostID() != host.HostID() {
		t.Fatalf("expected host %v got %v", host, *v)
	}

	next = iter()
	if next != nil {
		t.Errorf("expected to get nil host got %+v", next)
		if next.Info() == nil {
			t.Fatalf("HostInfo is nil")
		}
	}
}

func TestCOWList_Add(t *testing.T) {
	var cow cowHostList

	toAdd := [...]string{"peer1", "peer2", "peer3"}

	for _, addr := range toAdd {
		if !cow.add(&HostInfo{peer: addr}) {
			t.Fatal("did not add peer which was not in the set")
		}
	}

	hosts := cow.get()
	if len(hosts) != len(toAdd) {
		t.Fatalf("expected to have %d hosts got %d", len(toAdd), len(hosts))
	}

	set := make(map[string]bool)
	for _, host := range hosts {
		set[host.Peer()] = true
	}

	for _, addr := range toAdd {
		if !set[addr] {
			t.Errorf("addr was not in the host list: %q", addr)
		}
	}
}
