package nan0

import (
	"testing"
	"time"
	"github.com/golang/protobuf/proto"
	"fmt"
	"io"
)

var nsDefaultPort int32 = 2324

func TestNan0_Close(t *testing.T) {
	ns := &Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "localhost",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	listener, err := ns.Start()
	if err != nil {
		t.Fail()
		fmt.Println(" \t\tTest Failed, ns.Start() failed \n\t\t")
		return
	}
	defer listener.Close()

	serviceMsg := proto.Clone(new(Service))
	n, err := ns.DialNan0(false, serviceMsg, 0,0)
	if n.closed == true {
		t.Fail()
		fmt.Println(" \t\tTest Failed, n.closed == true failed")
	}
	n.Close()
	if n.closed != true {
		t.Fail()
		fmt.Println(" \t\tTest Failed, n.closed != true after closed")
	}
}

func TestNan0_GetReceiver(t *testing.T) {
	ns := &Service{
		ServiceName: "TestService",
		Port:        nsDefaultPort,
		HostName:    "127.0.0.1",
		ServiceType: "Test",
		StartTime:   time.Now().Unix(),
	}
	listener, err := ns.Start()
	if err != nil {
		t.Fail()
		fmt.Println(" \t\tTest Failed, ns.Start() failed")
	}
	defer listener.Close()
	go func() {
		conn, _ := listener.Accept()
		for ; ; {
			io.Copy(conn, conn)
		}

	}()
	serviceMsg := proto.Clone(new(Service))
	n, err := ns.DialNan0(true, serviceMsg, 0,0)
	if err != nil {
		t.Fail()
		fmt.Println("\t\tTest Failed, Nan0 failed to connect to service")
	}
	defer n.Close()
	sender := n.GetSender()
	receiver := n.GetReceiver()
	sender <- ns
	waitingVal := <-receiver
	if waitingVal.String() != ns.String() {
		t.Fail()
		fmt.Printf(" \t\tTest Failed, \n\t\tsent %v, \n\t\treceived: %v\n", ns, waitingVal)
	} else {
		fmt.Println("TestNan0_GetReceiver Passed")
	}
}
