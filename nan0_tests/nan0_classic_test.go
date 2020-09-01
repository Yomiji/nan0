package nan0_tests

import (
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/yomiji/nan0/v2"
	"google.golang.org/protobuf/proto"
)

func TestNan0_Close(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}

	server, err := ns.NewNanoBuilder().
		BuildNanoServer(
			nan0.ToggleWriteDeadline(false),
		)
	if err != nil {
		t.Fatalf(" \t\tTest Failed, error: %v\n", err)
	}
	defer server.Shutdown()

	serviceMsg := proto.Clone(new(nan0.Service))
	n, err := ns.NewNanoBuilder().
		BuildNanoClient(
			nan0.AddMessageIdentity(serviceMsg),
			nan0.ToggleWriteDeadline(false),
		)
	if err != nil {
		t.Fatalf(" \t\tTest Failed, error: %v\n", err)
	}
	if n.IsClosed() == true {
		t.Fatal(" \t\tTest Failed, n.closed == true failed")
	}
	n.Close()
	if n.IsClosed() != true {
		t.Fatal(" \t\tTest Failed, n.closed != true after closed")
	}
}
func TestNan0_GetReceiver(t *testing.T) {
	// create the service configuration
	serviceConf := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	// message types need to be registered before used so add a new one
	server, err := serviceConf.NewNanoBuilder().
		BuildNanoServer(
			nan0.AddMessageIdentity(new(nan0.Service)),
		)
	if err != nil {
		t.Fatalf(" \t\tTest Failed, error: %v\n", err)
	}
	// remember to ALWAYS shut your server down when finished
	defer server.Shutdown()
	// This server is configured to read a value and echo that value back out
	go func() {
		conn := <-server.GetConnections()
		select {
		case msg, ok := <-conn.GetReceiver():
			if ok {
				conn.GetSender() <- msg
			}
		}
	}()

	// server and clients can use the same builder with different finalizer methods
	client, err := serviceConf.NewNanoBuilder().
		BuildNanoClient(
			nan0.AddMessageIdentity(new(nan0.Service)),
		)
	if err != nil {
		t.Fatal("\t\tTest Failed, Nan0 failed to connect to service")
	}
	// ALWAYS close your client
	defer client.Close()
	sender := client.GetSender()
	receiver := client.GetReceiver()
	// senders and receivers naturally block, unless you set a buffer value
	// they can also be used with 'select' statements for non-blocking communication
	sender <- serviceConf
	waitingVal := <-receiver

	if waitingVal.(*nan0.Service).String() != serviceConf.String() {
		t.Fatalf(" \t\tTest Failed, \n\t\tsent %v, \n\t\treceived: %v\n", serviceConf, waitingVal)
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

	builder := ns.NewNanoBuilder()
	server, err := builder.BuildNanoServer(
		nan0.AddMessageIdentity(proto.Clone(new(any.Any))),
		nan0.ToggleWriteDeadline(true),
	)
	if err != nil {
		t.Fatal("\t\tTest Failed BuildServer failed")
	}
	defer server.Shutdown()
	StartTestServerThread(server)

	n, err := builder.BuildNanoClient()
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

	builder1 := ns.NewNanoBuilder()
	server, err := builder1.BuildNanoServer(
		nan0.ToggleWriteDeadline(true),
		nan0.AddMessageIdentities(
			proto.Clone(new(nan0.Service)),
			proto.Clone(new(any.Any)),
		),
	)
	if err != nil {
		t.Fatal("\t\tTest Failed BuildServer failed")
	}

	defer server.Shutdown()
	StartTestServerThread(server)

	builder2 := ns.NewNanoBuilder()
	n, err := builder2.BuildNanoClient(
		nan0.ToggleWriteDeadline(true),
		nan0.AddMessageIdentities(
			proto.Clone(new(nan0.Service)),
			proto.Clone(new(any.Any)),
		),
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
		if _, ok := val.(any.Any); ok {
			t.Fatal("\t\tTest Failed, Nan0 should not be Any")
		}
		n.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("\t\tTest Failed, Timeout")
	}
}
func TestNan0_RepeatClient(t *testing.T) {
	// create the service configuration
	serviceConf := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	// message types need to be registered before used so add a new one
	server, err := serviceConf.NewNanoBuilder().
		BuildNanoServer(
			nan0.AddMessageIdentity(new(nan0.Service)),
		)
	if err != nil {
		t.Fatalf(" \t\tTest Failed, error: %v\n", err)
	}
	// remember to ALWAYS shut your server down when finished
	defer server.Shutdown()
	// This server is configured to read a value and echo that value back out
	go func() {
		for conn := range server.GetConnections() {
			select {
			case msg, ok := <-conn.GetReceiver():
				if ok {
					conn.GetSender() <- msg
				}
			}
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		client, err := serviceConf.NewNanoBuilder().
			BuildNanoClient(
				nan0.AddMessageIdentity(new(nan0.Service)),
			)
		if err != nil {
			t.Fatal("\t\tTest Failed, Nan0 failed to connect to service")
		}
		sender := client.GetSender()
		receiver := client.GetReceiver()
		sender <- serviceConf
		select {
		case waitingVal := <-receiver:
			if waitingVal.(*nan0.Service).String() != serviceConf.String() {
				t.Fatalf(" \t\tTest Failed, \n\t\tsent %v, \n\t\treceived: %v\n", serviceConf, waitingVal)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("Test timeout")
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		client, err := serviceConf.NewNanoBuilder().
			BuildNanoClient(
				nan0.AddMessageIdentity(new(nan0.Service)),
			)
		if err != nil {
			t.Fatal("\t\tTest Failed, Nan0 failed to connect to service")
		}
		sender := client.GetSender()
		receiver := client.GetReceiver()
		sender <- serviceConf
		select {
		case waitingVal := <-receiver:
			if waitingVal.(*nan0.Service).String() != serviceConf.String() {
				t.Fatalf(" \t\tTest Failed, \n\t\tsent %v, \n\t\treceived: %v\n", serviceConf, waitingVal)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("Test timeout")
		}
	}()
	wg.Wait()
}
func TestNan0_GetAllClients(t *testing.T) {
	// create the service configuration
	serviceConf := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	// message types need to be registered before used so add a new one
	server, err := serviceConf.NewNanoBuilder().
		BuildNanoServer(
			nan0.AddMessageIdentity(new(nan0.Service)),
			nan0.Route(nil, TestRoute{}),
		)
	if err != nil {
		t.Fatalf(" \t\tTest Failed, error: %v\n", err)
	}
	// remember to ALWAYS shut your server down when finished
	defer server.Shutdown()

	client, err := serviceConf.NewNanoBuilder().
		BuildNanoClient(
			nan0.AddMessageIdentity(new(nan0.Service)),
		)
	if err != nil {
		t.Fatal("\t\tTest Failed, Nan0 failed to connect to service")
	}
	client.GetSender() <- serviceConf
	<-client.GetReceiver()
	if n := len(server.GetAllConnections()); n != 1 {
		t.Fatalf("Expected 1 client, got %d", n)
	}
}