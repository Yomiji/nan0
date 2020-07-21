# Nan0
### Protobuf Nanoservice Framework for Go
##### Purpose
This framework is designed to make it easier to pass messages between nanoservices, which are just a fancy name for
lightweight microservices that compose some of my work. This framework supports developers in creating stream-based communications for protocol buffers over raw tcp connections.
The key features of this framework include:

* Security in-transit with minimal configuration

* Service discovery capabilities to connect peers without configuring IP addresses
* Quickly establish server and client communication objects using distinct pattern
* Pass protobuf-based objects directly into a channel to both clients and servers

##### Usage
When I put together the framework, I needed a way to build the services without having to worry about the network
transport protocols, message handshaking and so forth. Here are the primary uses and caveats:

* There is a logging pattern in place for this plugin, to configure it, follow the steps in **Logging** section of this
  readme or simply call the NoLogging function to disable logging from this framework:
```go
    package main
    import "github.com/yomiji/slog"
    
    func init() {
        slog.NoLogging()
    }
```
* The **Service** type defines the actual nanoservices that you can create to serve as a client OR server. In order to quickly establish a Service, just instantiate a Service object and create a builder for it. You will still need to handle passing data to the connections that are established via the returned object like so:
```go
    package main
          
    import (
          "github.com/yomiji/nan0/v2"
          "time"
          "fmt"
          "google.golang.org/protobuf/proto"
      )
      
    func TestNan0_GetReceiver(t *testing.T) {
        // create the service configuration
        serviceConf := &nan0.Service{
            ServiceName: "TestService",
            Port:        nsDefaultPort,
            HostName:    "127.0.0.1",
            ServiceType: "Test",
            StartTime:   time.Now().Unix(),
        }
        // message types need to be registered before used so add a new one
        server, err := serviceConf.NewNanoBuilder().
            BuildNanoServer(
                nan0.AddMessageIdentity(new(nan0.Service)),
            )
        if err != nil {
            t.Fatalf(" \t\tTest Failed, error: %v\n", err)
        }
        // remember to ALWAYS shut your server down when finished
        defer server.Shutdown()
        // This server is configured to read a value and echo that value back out
        go func() {
            conn := <-server.GetConnections()
            select {
            case msg,ok := <-conn.GetReceiver():
                if ok {
                    conn.GetSender() <- msg
                }
            }
        }()
    
        // server and clients can use the same builder with different finalizer methods
        client, err := serviceConf.NewNanoBuilder().
            BuildNanoClient(
                nan0.AddMessageIdentity(new(nan0.Service)),
            )
        if err != nil {
            t.Fatal("\t\tTest Failed, Nan0 failed to connect to service")
        }
        // ALWAYS close your client
        defer client.Close()
        sender := client.GetSender()
        receiver := client.GetReceiver()
        // senders and receivers naturally block, unless you set a buffer value
        // they can also be used with 'select' statements for non-blocking communication
        sender <- serviceConf
        waitingVal := <-receiver
    
        if waitingVal.(*nan0.Service).String() != serviceConf.String() {
            t.Fatalf(" \t\tTest Failed, \n\t\tsent %v, \n\t\treceived: %v\n", serviceConf, waitingVal)
        }
    }
````



#### Logging
Slog is a logging package that originated in [Nan0](github.com/yomiji/nan0). Slog is used to inform the consumer of the details of operation. The logging mechanism is remotely similar to some packages in other languages such as Java's slf4j; or at least I'd like it to be similar to something most people have seen before. That being said there are some functions that must be discussed, as the default settings may be more verbose than you need.

* There are four logging levels: ***Debug***, ***Info***, ***Warn***, and ***Error***
* All of the logging levels are enabled by default, to disable them, you must set the corresponding logger.
    ```go
      package main
      
      import "github.com/yomiji/slog"
      
      func main() {
      	// set logging levels enabled/disabled
      	// info: true, warn: true, fail: true, debug: false
        slog.ToggleLogging(true, true, true, false)
        
        slog.Info("this is a test") //works, prints
        slog.Debug("not going to print") // won't print
      }
    ```
##### Demo Project
There is a demo project created using the Nan0 API called [Nan0Chat](https://github.com/Yomiji/nan0chat) . This is a chat application that utilizes the
features in Nan0 to communicate securely between a server and multiple clients.

##### Planned Features
* Document nan0 settings in Wiki
* Add godoc examples
