package nan0_tests

import (
	"strconv"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/yomiji/nan0/v2"
	"google.golang.org/protobuf/proto"
)

func TestWebsocketClient(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        wsDefaultPort,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
		Uri:         "/",
	}
	var err error
	wsBuilder := ns.NewWebsocketBuilder()
	wsServer, _ = wsBuilder.BuildWebsocketServer(
		nan0.AddMessageIdentity(proto.Clone(new(nan0.Service))),
		nan0.AddOrigins("localhost:"+strconv.Itoa(int(wsDefaultPort))),
	)

	defer wsServer.Shutdown()
	StartTestServerThread(wsServer)

	ns2 := &nan0.Service{
		ServiceName: "TestService2",
		Port:        wsDefaultPort,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
		Uri:         "/",
	}

	receiver, err := ns2.NewWebsocketBuilder().
		BuildWebsocketClient(nan0.AddMessageIdentity(new(nan0.Service)))
	if err != nil {
		t.Fatalf("Failed to establish client connection")
	}
	defer receiver.Close()

	receiver.GetSender() <- ns2

	select {
	case val := <-receiver.GetReceiver():
		if _, ok := val.(*nan0.Service); !ok {
			t.Fatal("\t\tTest Failed, Nan0 service type was not returned from ws")
		}
		if val.(*nan0.Service).HostName != ns2.HostName {
			t.Fatal("\t\tTest Failed, Values not validated")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("\t\tTest Failed, Timeout")
	}
}
func TestNan0_RouteWs(t *testing.T) {
	ns := &nan0.Service{
		Port:        wsDefaultPort,
		StartTime:   time.Now().Unix(),
		HostName:    "localhost",
		Uri:         "/",
		ServiceName: "TestService",
		ServiceType: "Test",
	}

	builder1 := ns.NewWebsocketBuilder()
	server, err := builder1.BuildWebsocketServer(
		nan0.AddOrigins("localhost:"+strconv.Itoa(int(wsDefaultPort))),
		nan0.AddMessageIdentities(new(nan0.Service)),
		nan0.Route(new(nan0.Service), new(TestRoute)),
	)
	if err != nil {
		t.Fatal("\t\tTest Failed BuildServer failed")
	}

	defer server.Shutdown()

	builder2 := ns.NewWebsocketBuilder()
	n, err := builder2.BuildWebsocketClient(
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
			t.Fatal("\t\tTest Failed, received should be nan0.Service")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("\t\tTest Failed, Timeout")
	}
}
func TestNan0_DefaultRouteWs(t *testing.T) {
	ns := &nan0.Service{
		Port:        wsDefaultPort,
		StartTime:   time.Now().Unix(),
		HostName:    "localhost",
		Uri:         "/",
		ServiceName: "TestService",
		ServiceType: "Test",
	}

	builder1 := ns.NewWebsocketBuilder()
	server, err := builder1.BuildWebsocketServer(
		nan0.AddOrigins("localhost:"+strconv.Itoa(int(wsDefaultPort))),
		nan0.ToggleWriteDeadline(true),
		nan0.AddMessageIdentities(new(nan0.Service), new(any.Any)),
		nan0.Route(nil, new(TestRoute)),
	)
	if err != nil {
		t.Fatal("\t\tTest Failed BuildServer failed")
	}

	defer server.Shutdown()

	builder2 := ns.NewWebsocketBuilder()
	n, err := builder2.BuildWebsocketClient(
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
func TestNan0_InvalidRouteWs(t *testing.T) {
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

	builder1 := ns.NewWebsocketBuilder()
	_, _ = builder1.BuildWebsocketServer(
		nan0.ToggleWriteDeadline(true),
		nan0.Route(nil, nil),
	)

	t.Fatal("\t\tTest Failed BuildServer succeeded when fail expected")
}
