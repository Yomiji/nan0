package nan0_tests

import (
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/yomiji/nan0/v2"
)

func TestNan0_Route(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}

	builder1 := ns.NewNanoBuilder()
	server, err := builder1.BuildNanoServer(
		nan0.ToggleWriteDeadline(true),
		nan0.Route(new(nan0.Service), new(TestRoute)),
	)
	if err != nil {
		t.Fatal("\t\tTest Failed BuildServer failed")
	}

	defer server.Shutdown()

	builder2 := ns.NewNanoBuilder()
	n, err := builder2.BuildNanoClient(
		nan0.ToggleWriteDeadline(true),
		nan0.AddMessageIdentity(new(nan0.Service)),
	)
	if err != nil {
		t.Fatal("\t\tTest Failed, Nan0 failed to connect to service")
	}
	defer n.Close()

	sender := n.GetSender()
	sender <- ns
	receiver := n.GetReceiver()

	select {
	case val := <-receiver:
		if _, ok := val.(*nan0.Service); !ok {
			t.Fatal("\t\tTest Failed, Nan0 should be nan0.Service")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("\t\tTest Failed, Timeout")
	}
}
func TestNan0_DefaultRoute(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}

	builder1 := ns.NewNanoBuilder()
	server, err := builder1.BuildNanoServer(
		nan0.ToggleWriteDeadline(true),
		nan0.AddMessageIdentities(new(nan0.Service), new(any.Any)),
		nan0.Route(nil, new(TestRoute)),
	)
	if err != nil {
		t.Fatal("\t\tTest Failed BuildServer failed")
	}

	defer server.Shutdown()

	builder2 := ns.NewNanoBuilder()
	n, err := builder2.BuildNanoClient(
		nan0.ToggleWriteDeadline(true),
		nan0.AddMessageIdentity(new(nan0.Service)),
		nan0.AddMessageIdentity(new(any.Any)),
	)
	if err != nil {
		t.Fatal("\t\tTest Failed, Nan0 failed to connect to service")
	}
	defer n.Close()

	sender := n.GetSender()
	sender <- ns
	receiver := n.GetReceiver()

	select {
	case val := <-receiver:
		if _, ok := val.(*nan0.Service); !ok {
			t.Fatal("\t\tTest Failed, received should be nan0.Service")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("\t\tTest Failed, Timeout")
	}
	sender <- new(any.Any)
	select {
	case val := <-receiver:
		if _, ok := val.(*any.Any); !ok {
			t.Fatal("\t\tTest Failed, received should be any.Any")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("\t\tTest Failed, Timeout")
	}
}
func TestNan0_InvalidRoute(t *testing.T) {
	defer func() {
		if v := recover(); v != nil {
			t.Log("Passed")
		}
	}()
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}

	builder1 := ns.NewNanoBuilder()
	_, _ = builder1.BuildNanoServer(
		nan0.ToggleWriteDeadline(true),
		nan0.Route(nil, nil),
	)

	t.Fatal("\t\tTest Failed BuildServer succeeded when fail expected")
}
