package nan0

import (
	"testing"
	"fmt"
	"time"
)

var (
	dsDefaultPort int32 = 4345

)
func TestNanoDiscoveryCreationWithPositiveResult(t *testing.T) {
	fmt.Println(">>> Running Nano Discovery Creation Test <<<")
	ds := NewDiscoveryService(dsDefaultPort, 0)

	if ds.defaultPort != dsDefaultPort {
		t.Fail()
	}

	ds.Shutdown()
	if !ds.IsShutdown() {
		t.Fail()
	}
}
func TestNanoDiscoveryCreationWithoutCorrectPort(t *testing.T) {
	fmt.Println(">>> Running Nano Discovery Creation with Incorrect Port Test <<<")
	ds := NewDiscoveryService(0, 0)

	fmt.Println("\t\tTesting default port check")
	if ds.defaultPort != 0 {
		t.Fail()
	}

	fmt.Println("\t\tShutting down")
	ds.Shutdown()
}

func TestNanoDiscoveryCanRegisterAService(t *testing.T) {
	fmt.Println(">>> Running Nano Discovery Can Register Service <<<")
	ds := NewDiscoveryService(dsDefaultPort, 0)
	ns := &Service{
		ServiceType:"Test",
		StartTime:time.Now().Unix(),
		ServiceName:"TestService",
		HostName:"localhost",
		Port:5555,
		TimeToLiveInMS: 60000,
	}

	//TODO: figure out a way to get this to finish before shutting down (block on this call)
	ns.Register("localhost", dsDefaultPort)

	//services should be the same
	if nsr := ds.GetServiceByName("TestService"); nsr == nil  {
		fmt.Printf("\t\tTest Failed, nsr == nil, \n\t\t nsr: %v \n\t\t ns: %v", nsr, ns)
		t.Fail()
	} else if nsr := ds.GetServiceByName("TestService"); !nsr.Equals(*ns) {
		fmt.Printf("\t\tTest Failed, nsr != ns, \n\t\t nsr: %v \n\t\t ns: %v", nsr, ns)
		t.Fail()
	}

	ds.Shutdown()
}