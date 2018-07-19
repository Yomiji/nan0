# Nan0
### Protobuf Nanoservice Framework for Go
##### Purpose
This framework is designed to make it easier to pass messages between nanoservices, which are just a fancy name for
lightweight microservices that compose some of my work.

When I put together the framework, I needed a way to build the services without having to worry about the network
transport protocols, message handshaking and so forth. That said, there are a few concepts that users of this framework
should be aware of before using it:

* There is a logging pattern in place for this plugin, to configure it, follow the steps in **Logging** section of this
  readme or simply call the NoLogging function to disable logging from this framework:
  ```go
    package main
    import "nan0"
    
    func init() {
        nan0.NoLogging()
    }
  ```
* The **Service** type defines the actual nanoservices that you can create to serve as a client OR server. In order to
  quickly establish a Service, just instantiate a Service object and call the member function Start on it. You will
  still need to take the listener returned from this function and create a server:
  ```go
        package main
        
        import (
        	"nan0"
        	"time"
        	"io"
      	    "fmt"
        )
        
        //NOTE: Error checking omitted for demonstration purposes only, PLEASE be more vigilant in production systems.
        func main() {
        	// Create a nan0 service
        	service := &nan0.Service{
        		ServiceName: "TestService",
        		Port:        4546,
        		HostName:    "127.0.0.1",
        		ServiceType: "Test",
        		StartTime:   time.Now().Unix(),
        	}
        	
        	// Start a server
        	listener, _ := service.Start()
        	
        	// Shutdown when finished
        	defer listener.Close()
        	
        	// Create an echo service, every protocol buffer sent to this service will be echoed back
        	go func() {
        		conn, _ := listener.Accept()
        		for ; ; {
        			io.Copy(conn, conn)
        		}
        	}()
        	
        	// Establish a client connection
        	comm,_ := service.DialNan0(false, service, 0, 0)
        	
        	// Shutdown when finished
        	defer comm.Close()
        	
        	// The nan0.Nan0 allows for sending and receiving protobufs on channels for communication
        	sender := comm.GetSender()
        	receiver := comm.GetReceiver()
        	
        	// Send a protocol buffer, yes nan0.Service is a protobuf type
        	sender <- service
        	// Wait to receive a response, which should be the Service back again in this case due to the echo code above
        	result := <-receiver
        	
        	// Test the results, should be the same
        	if service.String() == result.String() {
        		fmt.Println("Service was echoed back")
      	}
        }
  ```
* The **DiscoveryService** type defines a server that registers several nanoservices under a specified type and by name.
  ```go package main
    
    import (
        "nan0"
        "time"
        "net"
        "io/ioutil"
        "github.com/golang/protobuf/proto"
    )
    
    func main() {
        // Create a DiscoveryService object
        discoveryService := nan0.NewDiscoveryService(4677, 10)
        
        // Remember to shutdown
        defer discoveryService.Shutdown()
    
        // Create a nanoservice
        ns := &nan0.Service{
            ServiceType:"Test",
            StartTime:time.Now().Unix(),
            ServiceName:"TestService",
            HostName:"::1",
            Port:5555,
            Expired: false,
        }
    
        // Register the service with the discovery service
        ns.Register("::1", 4677)
        
        // Connect to discoveryService and read to fill ServiceList structure to get all services
        conn, _ := net.Dial("tcp", "[::1]:4677")
        b,_ := ioutil.ReadAll(conn)
        services := &nan0.ServiceList{}
        proto.Unmarshal(b, services)
        
        // Do something with available services
        _ = services.ServicesAvailable
    }
    ```
