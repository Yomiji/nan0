package nan0_tests

import (
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/yomiji/nan0"
)

var nsDefaultPort int32 = 2324
var wsDefaultPort int32 = 8080


var wsServer nan0.Server

func startTestServerThread(wsServer nan0.Server) {
	go func() {
		conn := <-wsServer.GetConnections()
		select {
		case msg,ok := <-conn.GetReceiver():
			if ok {
				conn.GetSender() <- msg
			}
		}
	}()
}

func TestMain(m *testing.M) {
	//nan0.Debug = log.New(os.Stdout, "Nan0 [DEBUG]: ", log.Ldate|log.Ltime)
	//nan0.ToggleLineNumberPrinting(true, true, true, true)

	os.Exit(m.Run())
}

func TestNan0_Close(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	listener, err := ns.Start()
	if err != nil {
		t.Fatal(" \t\tTest Failed, ns.Start() failed \n\t\t")
	}
	defer listener.Close()

	serviceMsg := proto.Clone(new(nan0.Service))
	n, err := ns.NewNanoBuilder().
		AddMessageIdentity(serviceMsg).
		ToggleWriteDeadline(false).
		Build()

	if n.IsClosed() == true {
		t.Fatal(" \t\tTest Failed, n.closed == true failed")
	}
	n.Close()
	if n.IsClosed() != true {
		t.Fatal(" \t\tTest Failed, n.closed != true after closed")
	}
}

func TestNan0_GetReceiver(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	listener, err := ns.Start()
	defer listener.Close()

	if err != nil {
		t.Fatal(" \t\tTest Failed, ns.Start() failed")
	}
	go func() {
		conn, _ := listener.Accept()
		for ; ; {
			io.Copy(conn, conn)
		}

	}()
	serviceMsg := proto.Clone(new(nan0.Service))
	n, err := ns.NewNanoBuilder().
		AddMessageIdentity(serviceMsg).
		ToggleWriteDeadline(true).
		Build()
	if err != nil {
		t.Fatal("\t\tTest Failed, Nan0 failed to connect to service")
	}
	defer n.Close()
	sender := n.GetSender()
	receiver := n.GetReceiver()
	sender <- ns
	waitingVal := <-receiver
	if waitingVal.(proto.Message).String() != ns.String() {
		t.Fatalf(" \t\tTest Failed, \n\t\tsent %v, \n\t\treceived: %v\n", ns, waitingVal)
	}
}

func TestNan0_FailWithWrongType(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}

	builder := ns.NewNanoBuilder().
		AddMessageIdentity(proto.Clone(new(any.Any))).
		ToggleWriteDeadline(true)
	server, err := builder.BuildServer(nil)
	if err != nil {
		t.Fatal("\t\tTest Failed BuildServer failed")
	}

	defer server.Shutdown()
	startTestServerThread(server)

	n, err := builder.Build()
	defer n.Close()

	if err != nil {
		t.Fatal("\t\tTest Failed, Nan0 failed to connect to service")
	}
	sender := n.GetSender()
	sender <- ns
	receiver := n.GetReceiver()

	select {
	case <-receiver:
		t.Fatal("\t\tTest Failed, Nan0 should not have received anything")
	case <-time.After(2 * time.Second):
		t.Log("Passed!")
	}
}

func TestNan0_MixedOrderMessageIdent(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}

	builder1 := ns.NewNanoBuilder().
		AddMessageIdentities(proto.Clone(new(nan0.Service)), proto.Clone(new(any.Any))).
		ToggleWriteDeadline(true)
	server, err := builder1.BuildServer(nil)
	if err != nil {
		t.Fatal("\t\tTest Failed BuildServer failed")
	}

	defer server.Shutdown()
	startTestServerThread(server)


	builder2 := ns.NewNanoBuilder().
		AddMessageIdentities(proto.Clone(new(any.Any)), proto.Clone(new(nan0.Service))).
		ToggleWriteDeadline(true)
	n, err := builder2.Build()

	if err != nil {
		t.Fatal("\t\tTest Failed, Nan0 failed to connect to service")
	}
	defer n.Close()

	sender := n.GetSender()
	sender <- ns
	receiver := n.GetReceiver()

	select {
	case val := <-receiver:
		if _, ok := val.(any.Any); ok {
			t.Fatal("\t\tTest Failed, Nan0 should not be Any")
		}
		n.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("\t\tTest Failed, Timeout")
	}
}

func TestWebsocketClient(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        wsDefaultPort,
		HostName:    "",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
		Uri:         "/",
	}
	var err error
	wsBuilder := ns.NewNanoBuilder().
		Websocket().
		AddMessageIdentity(proto.Clone(new(nan0.Service))).
		AddOrigins("localhost:"+strconv.Itoa(int(wsDefaultPort)))
	wsServer, _ = wsBuilder.BuildServer(nil)

	defer wsServer.Shutdown()
	startTestServerThread(wsServer)


	ns2 := &nan0.Service{
		ServiceName: "TestService2",
		Port:        wsDefaultPort,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
		Uri:         "/",
	}

	receiver, err := ns2.NewNanoBuilder().
		Websocket().
		AddMessageIdentity(new(nan0.Service)).
		Build()
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
