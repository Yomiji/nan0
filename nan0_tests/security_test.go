package nan0_tests

import (
	"context"
	"crypto/tls"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/yomiji/nan0/v2"
	"google.golang.org/protobuf/proto"
)

func TestSecurity_SecurityEstablished(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort - 1,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}

	server, err := ns.NewNanoBuilder().
		BuildNanoServer(
			nan0.Secure,
		)
	if err != nil {
		t.Fatalf(" \t\tTest Failed, error: %v\n", err)
	}
	defer server.Shutdown()

	n, err := ns.NewNanoBuilder().
		BuildNanoClient(
			nan0.Secure,
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

func TestSecurity_SecureObjectSent(t *testing.T) {
	nsServer := &nan0.Service{
		ServiceName: "TestServer",
		Port:        nsDefaultPort - 8,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	nsClient := &nan0.Service{
		ServiceName: "TestServerRemote",
		Port:        nsDefaultPort - 8,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}

	server, err := nsServer.NewNanoBuilder().
		BuildNanoServer(
			nan0.Route(new(nan0.Service), new(TestRoute)),
			nan0.Secure,
		)
	if err != nil {
		t.Fatalf(" \t\tTest Failed, error: %v\n", err)
	}
	defer server.Shutdown()

	n, err := nsClient.NewNanoBuilder().
		BuildNanoClient(
			nan0.AddMessageIdentity(new(nan0.Service)),
			nan0.Secure,
		)
	if err != nil {
		t.Fatalf(" \t\tTest Failed, error: %v\n", err)
	}
	defer n.Close()
	n.GetSender() <- nsServer
	select {
	case obj := <-n.GetReceiver():
		if _, ok := obj.(*nan0.Service); !ok {
			t.Fatalf("expected object type *nan0.service, got %v", reflect.TypeOf(obj))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Test failed: timeout")
	}
}

func TestSecurity_InsecureClientCannotConnect(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort + 10,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	serviceMsg := proto.Clone(new(nan0.Service))

	server, err := ns.NewNanoBuilder().
		BuildNanoServer(
			nan0.AddMessageIdentity(serviceMsg),
			nan0.ToggleWriteDeadline(true),
		    nan0.MaxIdleDuration(100 * time.Millisecond),
			nan0.Secure,
		)
	if err != nil {
		t.Fatalf(" \t\tTest Failed, error: %v\n", err)
	}
	defer server.Shutdown()
	//defer func() {
	//	recover()
	//	slog.Info("Passed due to panic created")
	//
	//}()
	StartTestServerThread(server)
	n, err := ns.NewNanoBuilder().
		BuildNanoClient(
			nan0.AddMessageIdentity(serviceMsg),
			nan0.ToggleWriteDeadline(true),
		    nan0.MaxIdleDuration(100 * time.Millisecond),
			nan0.Insecure,
		)
	if err != nil {
		t.Fatalf("\nExpected nil error and client created, got: %v", err)
	}
	defer n.Close()

	closed := make(chan struct{})
	go func() {
		for !n.IsClosed() {
			time.Sleep(1 * time.Millisecond)
		}
		closed <- struct{}{}
	}()
	select {
	case <-closed:
		t.Log("Passed: Connection closed")
	case <-time.After(3*time.Second):
		t.Fatal("Not expected a result")
	}
}

func TestSecurity_ServiceDiscoverySuccess(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	builder := ns.NewNanoBuilder()
	server, err := builder.BuildNanoServer(
		nan0.AddMessageIdentity(new(nan0.Service)),
		nan0.ServiceDiscovery,
		nan0.Secure,
	)
	if err != nil {
		t.FailNow()
	}
	StartTestServerThread(server)
	defer server.Shutdown()
	client, err := builder.BuildNanoDNS(context.Background(), nan0.WithTimeout(5*time.Second))(true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.GetSender() <- ns
	if v, ok := (<-client.GetReceiver()).(*nan0.Service); ok && v.StartTime != ns.StartTime {
		t.Fatal("Not equal")
	}
}

func TestSecurity_DirectWebsocketClient(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        wsDefaultPort,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
		Uri:         "/",
	}
	var err error
	//load security files
	wsServer, _ = ns.NewWebsocketBuilder().BuildWebsocketServer(
		nan0.AddMessageIdentity(proto.Clone(new(nan0.Service))),
		nan0.AddOrigins("localhost:"+strconv.Itoa(int(wsDefaultPort))),
		nan0.SecureWs(nan0.TLSConfig{
			CertFile: "./cert.pem",
			KeyFile:  "./key.pem",
		}),
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

	receiver, err := ns2.NewWebsocketBuilder().BuildWebsocketClient(
		nan0.AddMessageIdentity(new(nan0.Service)),
		nan0.SecureWs(nan0.TLSConfig{
			Config: tls.Config{RootCAs: nil, InsecureSkipVerify: true},
		}),
	)
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

func TestSecurity_SecureDiscoveryWebsocket(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "WebsocketDiscoveryService",
		Port:        wsDefaultPort,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
		Uri:         "/",
	}
	var err error
	wsServer, _ = ns.NewWebsocketBuilder().BuildWebsocketServer(
		nan0.AddMessageIdentity(proto.Clone(new(nan0.Service))),
		nan0.AddOrigins("localhost:"+strconv.Itoa(int(wsDefaultPort))),
		nan0.ServiceDiscovery,
		nan0.SecureWs(nan0.TLSConfig{
			CertFile: "./cert.pem",
			KeyFile:  "./key.pem",
		}),
	)
	defer wsServer.Shutdown()
	StartTestServerThread(wsServer)

	clientConfig := &nan0.Service{
		ServiceName: "WebsocketDiscoveryService",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	ctx, cancelFunc := context.WithCancel(context.Background())

	client, err := clientConfig.NewWebsocketBuilder().
		BuildWebsocketDNS(ctx,
			nan0.WithTimeout(10*time.Second),
			nan0.AddMessageIdentity(new(nan0.Service)),
			nan0.SecureWs(nan0.TLSConfig{
				Config: tls.Config{RootCAs: nil, InsecureSkipVerify: true},
			}),
		)(true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer cancelFunc()

	client.GetSender() <- ns
	if v, ok := (<-client.GetReceiver()).(*nan0.Service); !ok || (ok && v.StartTime != ns.StartTime) {
		t.Fatal("Not equal")
	}
}
