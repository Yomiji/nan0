package nan0_tests

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/yomiji/nan0/v2"
	"google.golang.org/protobuf/proto"
)

func Test_Discovery_GetsClient(t *testing.T) {
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
		nan0.Route(nil, new(TestRoute)),
		nan0.ServiceDiscovery,
		nan0.Insecure,
	)
	if err != nil {
		t.FailNow()
	}
	defer server.Shutdown()
	client, err := ns.NewNanoBuilder().BuildNanoDNS(
		context.Background(),
		nan0.WithTimeout(5*time.Second),
		nan0.AddMessageIdentity(new(nan0.Service)),
		nan0.Insecure)(true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.GetSender() <- ns
	select {
	case received := <-client.GetReceiver():
		if v, ok := (received).(*nan0.Service); ok && v.StartTime != ns.StartTime {
			t.Fatal("Not equal")
		}
	case <-time.After(testTimeout):
		t.Fatal("Test Timeout")
	}
}

func Test_Discovery_ConnectionFactoryPattern(t *testing.T) {
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
		nan0.Route(nil, new(TestRoute)),
		nan0.ServiceDiscovery,
		nan0.Insecure,
	)
	if err != nil {
		t.FailNow()
	}
	defer server.Shutdown()
	connectionFactory := ns.NewNanoBuilder().BuildNanoDNS(
		context.Background(),
		nan0.WithTimeout(5*time.Second),
		nan0.AddMessageIdentity(new(nan0.Service)),
		nan0.Insecure)

	client, err := connectionFactory(true)
	if err != nil {
		t.Fatal(err)
	}
	client.GetSender() <- ns
	select {
	case received := <-client.GetReceiver():
		if v, ok := (received).(*nan0.Service); ok && v.StartTime != ns.StartTime {
			t.Fatal("Not equal")
		}
	case <-time.After(testTimeout):
		t.Fatal("Test Timeout")
	}
	client.Close()
	client, err = connectionFactory(true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.GetSender() <- ns
	select {
	case received := <-client.GetReceiver():
		if v, ok := (received).(*nan0.Service); ok && v.StartTime != ns.StartTime {
			t.Fatal("Not equal")
		}
	case <-time.After(testTimeout):
		t.Fatal("Test Timeout")
	}
}


func Test_Discovery_InvalidServerHostname(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "Blahblahblah",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	builder := ns.NewNanoBuilder()
	server, err := builder.BuildNanoServer(
		nan0.AddMessageIdentity(new(nan0.Service)),
		nan0.Route(nil, new(TestRoute)),
		nan0.ServiceDiscovery,
		nan0.Insecure,
	)
	if err != nil {
		t.FailNow()
	}
	defer server.Shutdown()
	_, err = builder.BuildNanoDNS(context.Background(), nan0.WithTimeout(testTimeout))(true)
	if err == nil {
		t.Fatal("Unexpected success when connecting to invalid hostname")
	} else {
		t.Logf("error: %v", err)
	}
}

func Test_Discovery_MultipleServices(t *testing.T) {
	nsService1 := &nan0.Service{
		ServiceName: "FirstService",
		Port:        nsDefaultPort + 1,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	nsService2 := &nan0.Service{
		ServiceName: "SecondService",
		Port:        nsDefaultPort + 2,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	nsClient := &nan0.Service{
		ServiceName: "FirstService",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	s1, err := BuildServer(nsService1)
	if err != nil {
		t.Fatalf("Service 1 failed to start")
	}
	defer s1.Shutdown()
	s2, err := BuildServer(nsService2)
	if err != nil {
		t.Fatalf("Service 2 failed to start")
	}
	defer s2.Shutdown()

	//Connect to service 1 and send data
	clientBuilder := nsClient.NewNanoBuilder()

	ctx, cancelFunc := context.WithCancel(context.Background())
	client, err := clientBuilder.BuildNanoDNS(ctx,
		nan0.WithTimeout(5*time.Second),
		nan0.AddMessageIdentity(new(nan0.Service)),
		nan0.Insecure,
	)(true)
	if err != nil {
		t.Fatal(err)
	}
	client.GetSender() <- nsClient
	select {
	case received := <-client.GetReceiver():
		if v, ok := (received).(*nan0.Service); ok && v.StartTime != nsClient.StartTime {
			t.Fatal("Not equal")
		}
	case <-time.After(testTimeout):
		t.Fatal("Test Timeout")
	}
	client.Close()
	cancelFunc()

	ctx, cancelFunc = context.WithCancel(context.Background())
	defer cancelFunc()
	//Connect to service 1 and send data
	nsClient.ServiceName = nsService2.ServiceName
	nsClient.ServiceType = nsService2.ServiceType
	clientBuilder = nsClient.NewNanoBuilder()
	client, err = clientBuilder.BuildNanoDNS(ctx,
		nan0.WithTimeout(5*time.Second),
		nan0.AddMessageIdentity(new(nan0.Service)),
		nan0.Insecure,
	)(true)
	if err != nil {
		t.Fatal(err)
	}
	client.GetSender() <- nsClient
	select {
	case received := <-client.GetReceiver():
		if v, ok := (received).(*nan0.Service); ok && v.StartTime != nsClient.StartTime {
			t.Fatal("Not equal")
		}
	case <-time.After(testTimeout):
		t.Fatal("Test Timeout")
	}
}
func TestDiscovery_Websocket(t *testing.T) {
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
		nan0.Route(nil, new(TestRoute)),
	)
	defer wsServer.Shutdown()

	clientConfig := &nan0.Service{
		ServiceName: "WebsocketDiscoveryService",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	ctx, cancelFunc := context.WithCancel(context.Background())

	client, err := clientConfig.NewWebsocketBuilder().
		BuildWebsocketDNS(ctx,
			nan0.WithTimeout(5*time.Second),
			nan0.AddMessageIdentity(new(nan0.Service)),
			nan0.Insecure,
		)(true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer cancelFunc()

	client.GetSender() <- ns
	select {
	case received := <-client.GetReceiver():
		if v, ok := (received).(*nan0.Service); ok && v.StartTime != ns.StartTime {
			t.Fatal("Not equal")
		}
	case <-time.After(testTimeout):
		t.Fatal("Test Timeout")
	}
}
func BuildServer(ns *nan0.Service) (nan0.Server, error) {
	server, err := ns.NewNanoBuilder().BuildNanoServer(
		nan0.AddMessageIdentity(new(nan0.Service)),
		nan0.Route(nil, new(TestRoute)),
		nan0.ServiceDiscovery,
		nan0.Insecure,
	)
	if err != nil {
		return nil, err
	}
	return server, err
}
