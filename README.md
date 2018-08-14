# Nan0
### Protobuf Nanoservice Framework for Go
##### Purpose
This framework is designed to make it easier to pass messages between nanoservices, which are just a fancy name for
lightweight microservices that compose some of my work. This framework supports developers in creating stream-based communications for protocol buffers over raw tcp connections.

##### Usage
When I put together the framework, I needed a way to build the services without having to worry about the network
transport protocols, message handshaking and so forth. Here are the primary uses and caveats:

* There is a logging pattern in place for this plugin, to configure it, follow the steps in **Logging** section of this
  readme or simply call the NoLogging function to disable logging from this framework:
  ```go
    package main
    import "github.com/yomiji/nan0"
    
    func init() {
        nan0.NoLogging()
    }
  ```
* The **Service** type defines the actual nanoservices that you can create to serve as a client OR server. In order to quickly establish a Service, just instantiate a Service object and create a builder for it. You will still need to handle passing data to the connections that are established via the returned object like so:
  ```go
    package main
    
    import (
        "github.com/yomiji/nan0"
        "time"
        "fmt"
        "github.com/golang/protobuf/proto"
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
    
        // Create a service / client builder instance
        builder := service.NewNanoBuilder().
            AddMessageIdentity(proto.Clone(service)).
            ReceiveBuffer(0).
            SendBuffer(0)
        // Build an echo server, nil for default implementation
        server, _ := builder.BuildServer(nil)
        defer server.Shutdown()
        // The function for echo service, for the first connection, pass all messages received to the sender
        go func() {
            conn := <-server.GetConnections()
            for ; ; {
                select {
                case msg := <-conn.GetReceiver():
                    conn.GetSender() <- msg
                default:
                }
            }
        }()
    
        // Establish a client connection
        comm, _ := service.DialNan0().
            AddMessageIdentity(service).
            ToggleWriteDeadline(false).
            Build()
    
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
        if service.String() == result.(nan0.Service).String() {
            fmt.Println("Service was echoed back")
        }
    }
  ```
* You can create a secure service with authentication and encryption by creating ***Secret*** and ***Auth*** keys and
  calling the **DialNan0Secure** method. There are also DecryptProtobuf and EncryptProtobuf for use in server code.
  ```go
      package main
      
      import (
          "github.com/yomiji/nan0"
          "time"
      )
      
      // establish secrets and pass them to your server and client
      var secretKey = nan0.NewEncryptionKey()
      var authKey = nan0.NewHMACKey()
      
      func main() {
      	
      	// create a new service to connect to
          ns := &nan0.Service{
              ServiceName: "TestService",
              Port:        3234,
              HostName:    "127.0.0.1",
              ServiceType: "Test",
              StartTime:   time.Now().Unix(),
          }
          
          // use the resulting nanoservice connection
          ns.DialNan0Secure(secretKey, authKey).
              ToggleWriteDeadline(true).
              AddMessageIdentity(ns).
              SendBuffer(0).
              ReceiveBuffer(0).
          Build()
      }
  ```

* The **DiscoveryService** type defines a server that registers several nanoservices under a specified type and by name.
  Connect to the DiscoveryService using a Dial to obtain the list of Services registered.
  ```go
      package main
      
      import (
          "github.com/yomiji/nan0"
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
* The DiscoveryService returns a **ServiceList** object

##### Logging
This framework has a logging package that is used to inform the consumer of the details of operation. The logging
mechanism is remotely similar to some packages in other languages such as Java's slf4j; or at least I'd like it to be
similar to something most people have seen before. That being said there are some functions that must be discussed, as
the default settings may be more verbose than you need.

* There are four logging levels: ***Debug***, ***Info***, ***Warn***, and ***Error***
* All of the logging levels are enabled by default, to disable them, you must set the corresponding logger to ***nil***.
    ```go
      package main
      
      import "github.com/yomiji/nan0"
      
      func main() {
        nan0.Debug = nil
      }
    ```
* You can reassign the logger from console to another writer using the **SetLogWriter** function.
    ```go
      package main
      
      import (
        "github.com/yomiji/nan0"
        "net"
      )
      
      func main() {
        logserv,_ := net.Dial("tcp", "localhost:1234")
        nan0.SetLogWriter(logserv)
      }
    ```

##### Security Features
Security in Nan0 takes place at the connection level. The underlying network packets have the following structure
(constructed in the step order):

1. Message header preamble (by default 7 bytes)
2. Data Type identifier (8 bytes max)
3. HMAC size header (8 bytes max)
4. Size header (8 bytes max)
5. Encrypted bytes (length of "Size header" bytes)

Inside the encrypted bytes, a marshalled protobuf is signed using the HMAC key to provide a layer of authentication.

To utilize the encryption features, you must first create a private encryption key and an hmac authentication key, these
keys need to be used with the server and client.

```go
package main

import (
	"github.com/yomiji/nan0"
	"encoding/base64"
)

// A simple function to make the keys generated a sharable string
func shareKeys() (encKeyShare, authKeyShare string) {
  encKeyBytes := nan0.NewEncryptionKey()
  authKeyBytes := nan0.NewHMACKey()

  encKeyShare = base64.StdEncoding.EncodeToString(encKeyBytes[:])
  authKeyShare = base64.StdEncoding.EncodeToString(authKeyBytes[:])

  return
}

// The nan0 functions require a specific key type and width, this is a way to make 
// that conversion from strings to the required type.
//NOTE: Error checking omitted for demonstration purposes only, PLEASE be more vigilant in production systems.
func keysToNan0Bytes(encKeyShare, authKeyShare string) (encKey, authKey *[32]byte) {
  encKeyBytes, _ := base64.StdEncoding.DecodeString(encKeyShare)
  authkeyBytes, _ := base64.StdEncoding.DecodeString(authKeyShare)
  
  encKey = &[32]byte{}
  authKey = &[32]byte{}
  copy(encKey[:], encKeyBytes)
  copy(authKey[:], authkeyBytes)
  
  return
}
```

The server will utilize the supplied encryption key and signature to initialize the wrapper. The
encryption is mostly abstracted from clients, messages can be retrieved or sent on the streams without regard to the
encryption being enabled, however, you must use DialNan0Secure method on their client instead of DialNan0.
```go
  package main
  
  import (
  "github.com/yomiji/nan0"
  "time"
  "fmt"
  "github.com/golang/protobuf/proto"
  )
  
  func main() {
    // Create a nan0 service
    service := &nan0.Service{
      ServiceName: "TestService",
      Port:        4546,
      HostName:    "127.0.0.1",
      ServiceType: "Test",
      StartTime:   time.Now().Unix(),
    }
  
    // Trivially create encryption and signing keys
    encryptionKey := nan0.NewEncryptionKey()
    signature := nan0.NewHMACKey()
  
    // Create a service / client builder instance
    builder := service.NewNanoBuilder().
      AddMessageIdentity(proto.Clone(new(nan0.Service))).
      SendBuffer(0).
      ReceiveBuffer(0).
      ToggleWriteDeadline(true).
      EnableEncryption(encryptionKey, signature)
  
    // Build an echo server, nil for default implementation
    server, _ := builder.BuildServer(nil)
    defer server.Shutdown()
    // The function for echo service, for the first connection
    // pass all messages received to the sender
    go func() {
      conn := <-server.GetConnections()
      for ; ; {
        select {
        case msg := <-conn.GetReceiver():
          conn.GetSender() <- msg
        default:
        }
      }
    }()
  
    // Establish a secure client connection
    comm, _ := builder.Build()
  
    // Shutdown when finished
    defer comm.Close()
  
    // The nan0.Nan0 allows for sending and receiving protobufs on channels for communication
    sender := comm.GetSender()
    receiver := comm.GetReceiver()
  
    // Send a protocol buffer, yes nan0.Service is a protobuf type
    sender <- service
  
    // Wait to receive a response, which should be the service as sent above (we ignored the sender silently)
    result := <-receiver
  
    // Test the results, should be the same
    if service.String() == result.(*nan0.Service).String() {
      fmt.Println("Service was echoed back")
    }
  }
```

The security framework is based off of [cryptopasta](https://github.com/gtank/cryptopasta) so you should check this
project out for more information on the actual implementation of security methods.

##### Demo Project
There is a demo project created using the Nan0 API called [Nan0Chat](https://github.com/yomiji/nan0chat) . This is a chat application that utilizes the
features in Nan0 to communicate securely between a server and multiple clients.

##### Planned Features
* Create a separate configuration for writer timeouts
* Document nan0 settings
* Add godoc examples
