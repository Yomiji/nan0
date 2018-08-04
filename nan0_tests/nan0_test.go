package nan0_tests

import (
	"testing"
	"time"
	"github.com/golang/protobuf/proto"
	"io"
	"github.com/yomiji/nan0"
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
	n, err := ns.DialNan0(false, serviceMsg, 0,0)
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
	if err != nil {
		t.Fatal(" \t\tTest Failed, ns.Start() failed")
	}
	defer listener.Close()
	go func() {
		conn, _ := listener.Accept()
		for ; ; {
			io.Copy(conn, conn)
		}

	}()
	serviceMsg := proto.Clone(new(nan0.Service))
	n, err := ns.DialNan0(true, serviceMsg, 0,0)
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

func TestService_DialNan0Secure(t *testing.T) {
	ns := &nan0.Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	listener, err := ns.Start()
	if err != nil {
		t.Fatal(" \t\tTest Failed, ns.Start() failed")
	}
	defer listener.Close()
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
		MessageIdentity(serviceMsg).
		SendBuffer(0).
		ReceiveBuffer(0).
	BuildNan0()

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
