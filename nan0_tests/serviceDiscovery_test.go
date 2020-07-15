package nan0_tests

import (
	"context"
	"testing"
	"time"

	"github.com/yomiji/nan0"
)

func Test_Discovery_GetsClient(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	builder := ns.NewNanoBuilder().AddMessageIdentity(new(nan0.Service)).ServiceDiscovery(8000).Insecure()
	server,err := builder.BuildServer(nil)
	if err != nil {
		t.FailNow()
	}
	StartTestServerThread(server)
	defer server.Shutdown()
	client, err := builder.BuildNan0DNS(context.Background())(5 * time.Second, true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.GetSender() <- ns
	if v,ok := (<-client.GetReceiver()).(*nan0.Service); ok && v.StartTime != ns.StartTime {
		t.Fatal("Not equal")
	}
}
func Test_Discovery_MultipleServices(t *testing.T) {
	nsService1 := &nan0.Service{
		ServiceName: "FirstService",
		Port:        nsDefaultPort+1,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	nsService2 := &nan0.Service{
		ServiceName: "SecondService",
		Port:        nsDefaultPort+2,
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
	clientBuilder := nsClient.NewNanoBuilder().
		AddMessageIdentity(new(nan0.Service)).
		Insecure()
	ctx, cancelFunc := context.WithCancel(context.Background())
	client, err := clientBuilder.BuildNan0DNS(ctx)(5 * time.Second, true)
	if err != nil {
		t.Fatal(err)
	}
	client.GetSender() <- nsClient
	if v,ok := (<-client.GetReceiver()).(*nan0.Service); ok && v.StartTime != nsClient.StartTime {
		t.Fatal("Not equal")
	}
	client.Close()
	cancelFunc()

	//Connect to service 1 and send data
	nsClient.ServiceName = nsService2.ServiceName
	nsClient.ServiceType = nsService2.ServiceType
	clientBuilder = nsClient.NewNanoBuilder().
		AddMessageIdentity(new(nan0.Service)).
		Insecure()
	client, err = clientBuilder.BuildNan0DNS(context.Background())(5 * time.Second, true)
	if err != nil {
		t.Fatal(err)
	}
	client.GetSender() <- nsClient
	if v,ok := (<-client.GetReceiver()).(*nan0.Service); ok && v.StartTime != nsClient.StartTime {
		t.Fatal("Not equal")
	}
}

func BuildServer(ns *nan0.Service) (nan0.Server, error) {
	server,err := ns.NewNanoBuilder().
		AddMessageIdentity(new(nan0.Service)).
		ServiceDiscovery(8000).
		Insecure().
		BuildServer(nil)
	if err != nil {
		return nil, err
	}
	StartTestServerThread(server)
	return server, err
}