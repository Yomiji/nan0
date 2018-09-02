package nan0_tests

import (
	"testing"
	"time"
	"github.com/golang/protobuf/proto"
	"io"
	"github.com/yomiji/nan0"
	"github.com/golang/protobuf/ptypes/any"
	"fmt"
)

var nsDefaultPort int32 = 2324

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
	n, err := ns.DialNan0().
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
	n, err := ns.DialNan0().
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
	server,err := builder.BuildServer(nil)
	defer server.Shutdown()
	if err != nil {
		t.Fatal("\t\tTest Failed BuildServer failed")
	}
	go func() {
		conn := <-server.GetConnections()
		defer conn.Close()
		for ; ; {
			select {
			case msg := <-conn.GetReceiver():
					conn.GetSender() <- msg
			default:
			}
		}
	}()

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

func TestService_DialNan0Secure(t *testing.T) {
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
	encryptionKey := nan0.NewEncryptionKey()
	hmacKey := nan0.NewHMACKey()

	n, err := ns.DialNan0Secure(encryptionKey, hmacKey).
		ToggleWriteDeadline(true).
		AddMessageIdentity(serviceMsg).
		SendBuffer(0).
		ReceiveBuffer(0).
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

func Test_BuildServer(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	encryptionKey := nan0.NewEncryptionKey()
	hmacKey := nan0.NewHMACKey()

	builder := ns.DialNan0().
		AddMessageIdentity(proto.Clone(new(nan0.Service))).
		SendBuffer(1).
		ReceiveBuffer(1).
		ToggleWriteDeadline(true).
		EnableEncryption(encryptionKey, hmacKey)
	server,err := builder.BuildServer(nil)
	defer server.Shutdown()
	if err != nil {
		t.Fatal("\t\tTest Failed BuildServer failed")
	}
	go func() {
		conn := <-server.GetConnections()
		for !server.IsShutdown() {
			select {
			case msg := <-conn.GetReceiver():
				//if ok && msg != nil {
					conn.GetSender() <- msg
				//}
			default:
			}
		}
	}()
	client,err := builder.Build()
	if err != nil {
		t.Fatal("\t\tTest Failed, Failed to create client")
	}
	defer client.Close()

	fmt.Println("Sending msg")
	sender := client.GetSender()
	sender <- ns
	receiver := client.GetReceiver()

	select {
	case msg := <-receiver:
		t.Logf("Passed, received %v\n", msg)
	case <-time.After(2 * time.Second):
		t.Fatal("Failed, did not receive message within allotted time")
	}
}
