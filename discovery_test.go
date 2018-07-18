package nan0

import (
	"testing"
	"fmt"
	"time"
	"io"
)

var (
	dsDefaultPort int32 = 4345
)

//func TestMain(m *testing.M) {
//	noLogging()
//	os.Exit(m.Run())
//}


func TestNanoDiscoveryCreationWithoutCorrectPortShouldFail(t *testing.T) {
	fmt.Println(">>> Running Nano Discovery Creation with Incorrect Port Test <<<")
	ds := NewDiscoveryService(0, 0)

	fmt.Println("\t\tTesting default port check")
	if !ds.IsShutdown() {
		t.Fail()
	}
}

func TestNanoDiscoveryRegisterServiceShouldSucceed(t *testing.T) {
	fmt.Println(">>> Running Nano Discovery Register Servic  Should Succeed Test <<<")
	ds := NewDiscoveryService(dsDefaultPort, 0)
	defer ds.Shutdown()
	ns := &Service{
		ServiceType:"Test",
		StartTime:time.Now().Unix(),
		ServiceName:"TestService",
		HostName:"localhost",
		Port:5555,
		Expired: false,
	}

	ns.Register("127.0.0.1", dsDefaultPort)

	//get the service (may not be registered yet)
	nsr := ds.GetServiceByName("TestService")

	//wait til service is registered
	for nsr==nil {
		nsr = ds.GetServiceByName("TestService")
		time.Sleep(10 * time.Millisecond)
	}

	//services should be the same
	if  nsr == nil  {
		fmt.Printf("\t\tTest Failed, nsr == nil, \n\t\t nsr: %v \n\t\t ns: %v", nsr, ns)
		t.Fail()
	} else if nsr := ds.GetServiceByName("TestService"); !nsr.Equals(*ns) {
		fmt.Printf("\t\tTest Failed, nsr != ns, \n\t\t nsr: %v \n\t\t ns: %v", nsr, ns)
		t.Fail()
	}

}

func TestNanoDiscoveryRegisterMultipleServicesShouldSucceed(t *testing.T) {
	fmt.Println(">>> Running Nano Discovery Multiple Services Should Succeed Test <<<")
	ds := NewDiscoveryService(dsDefaultPort, 0)
	defer ds.Shutdown()
	ns1 := &Service{
		ServiceType:"Test",
		StartTime:time.Now().Unix(),
		ServiceName:"TestService1",
		HostName:"localhost",
		Port:5555,
		Expired: false,
	}
	ns2 := &Service {
		ServiceType:"Test",
		StartTime:time.Now().Unix(),
		ServiceName:"TestService2",
		HostName:"localhost",
		Port:5556,
		Expired: false,
	}

	ns1.Register("127.0.0.1", dsDefaultPort)
	ns2.Register("127.0.0.1", dsDefaultPort)

	//get the service (may not be registered yet)
	nses := ds.GetServicesByType("Test")

	deadline := time.Now().Add(10*time.Second)

	//wait til service is registered or timeout
	for len(nses) < 2 && time.Now().Before(deadline) {
		nses = ds.GetServicesByType("Test")
		time.Sleep(10 * time.Millisecond)
	}

	//services should be the same
	if  nses == nil  {
		fmt.Printf("\t\tTest Failed, nses == nil, \n\t\t nses: %v \n\t\t ns1: %v \n\t\t ns2: %v", nses, ns1, ns2)
		t.Fail()
	} else if len(nses) < 2 {
		fmt.Printf("\t\tTest Failed, nses < 2, \n\t\t nses: %v  \n\t\t ns1: %v \n\t\t ns2: %v", nses, ns1, ns2)
		t.Fail()
	}

}

func TestNanoDiscoveryRegisteredServicesTimeOut(t *testing.T) {
	fmt.Println(">>> Running Nano Discovery Registered Services Time Out Test <<<")
	ds := NewDiscoveryService(dsDefaultPort, 3)
	defer ds.Shutdown()
	// make a service that is inactive, no tcp, should be removed when service refresh time hits
	ns := &Service{
		ServiceType:"Test",
		StartTime:time.Now().Unix(),
		ServiceName:"TestService",
		HostName:"localhost",
		Port:5555,
		Expired:true,
	}


	// register the service
	ns.Register("localhost", dsDefaultPort)


	//get the service (may not be registered yet)
	nsr := ds.GetServiceByName("TestService")

	//wait til service is registered
	deadline := time.Now().Add(10*time.Second)
	for nsr==nil  && time.Now().Before(deadline) {
		nsr = ds.GetServiceByName("TestService")
		time.Sleep(10 * time.Millisecond)
	}

	//check that we have nsr as an actual object first
	if nsr == nil {
		fmt.Printf("\t\tTest Failed, nsr == nil, \n\t\t nsr: %v", nsr)
		t.Fail()
	}

	//wait for the ds to check the server to see if it is alive
	time.Sleep(3 * time.Second)

	// get the updated result from the service register, wait for nsr to be invalidated
	deadline = time.Now().Add(10*time.Second)
	for nsr !=nil  && time.Now().Before(deadline) {
		nsr = ds.GetServiceByName("TestService")
		time.Sleep(10 * time.Millisecond)
	}

	// if the service is still open according to the discoverer, we've failed the test
	if nsr != nil {
		fmt.Printf("\t\tTest Failed, nsr != nil, \n\t\t nsr: %v", nsr)
		t.Fail()
	}
}

func TestNanoDiscoveryGetServiceBytes(t *testing.T) {
	fmt.Println(">>> Running Nano Discovery Service Bytes Functions <<<")
	ds := NewDiscoveryService(dsDefaultPort, 0)
	defer ds.Shutdown()
	// make a service that is inactive, no tcp, should be removed when service refresh time hits
	ns := &Service{
		ServiceType:"Test",
		StartTime:time.Now().Unix(),
		ServiceName:"TestService",
		HostName:"localhost",
		Port:5555,
		Expired: false,
	}

	ns.Register("localhost", dsDefaultPort)

	//get the service (may not be registered yet)
	nsr := ds.GetServiceByName("TestService")

	//wait til service is registered
	deadline := time.Now().Add(10*time.Second)
	for nsr==nil  && time.Now().Before(deadline) {
		nsr = ds.GetServiceByName("TestService")
		time.Sleep(10 * time.Millisecond)
	}

	//get the bytes of the service and ensure they are present
	nsb, err := ds.GetServiceByNameBytes("TestService")
	if err != nil {
		fmt.Printf("\t\t GetServicesByTypeBytes Test Failed, err != nil, \n\t\t nsr: %v", nsr)
		t.Fail()
	}

	if nsb == nil {
		fmt.Printf("\t\t GetServiceByNameBytes Test Failed, nsb == nil, \n\t\t nsr: %v", nsr)
		t.Fail()
	}

	if len(nsb) <= 0 {
		fmt.Printf("\t\t GetServiceByNameBytes Test Failed, len(nsb) <= 0, \n\t\t nsr: %v", nsr)
		t.Fail()
	}

	nsb, err = ds.GetServicesByTypeBytes("Test")

	if err != nil {
		fmt.Printf("\t\t GetServicesByTypeBytes Test Failed, err != nil, \n\t\t nsr: %v", nsr)
		t.Fail()
	}

	if err != nil || nsb == nil {
		fmt.Printf("\t\t GetServicesByTypeBytes Test Failed, nsb == nil, \n\t\t nsr: %v", nsr)
		t.Fail()
	}

	if len(nsb) <= 0 {
		fmt.Printf("\t\t GetServicesByTypeBytes Test Failed, len(nsb) <= 0, \n\t\t nsr: %v", nsr)
		t.Fail()
	}

}

func TestNanoDiscoveryServiceCanBeSerializedUsingRead(t *testing.T) {
	fmt.Println(">>> Running Nano Discovery Service Can Be Serialized Using Read <<<")
	ds := NewDiscoveryService(dsDefaultPort, 0)
	defer ds.Shutdown()
	// make a service that is inactive, no tcp, should be removed when service refresh time hits
	ns1 := &Service{
		ServiceType:"Test",
		StartTime:time.Now().Unix(),
		ServiceName:"TestService1",
		HostName:"localhost",
		Port:5555,
		Expired: false,
	}
	ns2 := &Service{
		ServiceType:"Test",
		StartTime:time.Now().Unix(),
		ServiceName:"TestService2",
		HostName:"localhost",
		Port:5555,
		Expired: false,
	}
	ns1.Register("localhost", dsDefaultPort)
	ns2.Register("localhost", dsDefaultPort)

	nses := ds.GetServicesByType("Test")

	//wait til service is registered
	deadline := time.Now().Add(10*time.Second)
	for len(nses) < 2  && time.Now().Before(deadline) {
		nses = ds.GetServicesByType("TestService")
		time.Sleep(10 * time.Millisecond)
	}

	nds := NewDiscoveryService(dsDefaultPort+1, 0)
	defer nds.Shutdown()

	// copy from one discovery service to another
	io.Copy(nds, ds)

	ndsServices := nds.GetServicesByType("Test")
	if len(ndsServices) != 2 {
		t.Fail()
	}
}