package nan0

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yomiji/goprocrypt/v2"
	"google.golang.org/protobuf/proto"
)

const DefaultRoute = ""

type baseBuilderOption func(bb *baseBuilder)

type nanoClientFactory func(*baseBuilder) (NanoServiceWrapper, error)

type baseBuilder struct {
	ns                   *Service
	writeDeadlineActive  bool
	messageIdentMap      map[int]proto.Message
	inverseIdentMap      map[string]int
	routes               RouteMap
	serviceDiscovery     bool
	purgeConnections     time.Duration
	txRxIdleDuration     time.Duration
	maxServerConnections int
	sendBuffer           int
	receiveBuffer        int
	secure               bool
}

func (bb *baseBuilder) initialize(s *Service) {
	bb.messageIdentMap = make(map[int]proto.Message)
	bb.inverseIdentMap = make(map[string]int)
	bb.routes = make(RouteMap)
	bb.ns = s
}

func (bb *baseBuilder) build(opts ...baseBuilderOption) {
	for _, opt := range opts {
		opt(bb)
	}
}

// Flag indicating if service discovery is enabled (client/server)
func ServiceDiscovery(bb *baseBuilder) {
	bb.serviceDiscovery = true
}

// Flag indicating this builder is insecure
func Insecure(bb *baseBuilder) {
	bb.secure = false
}

// Flag indicating this builder is secure
// this will set up a secure handshake process on connection (tcp)
func Secure(bb *baseBuilder) {
	bb.secure = true
	AddMessageIdentities(
		proto.Clone(new(goprocrypt.PublicKey)),
		proto.Clone(new(goprocrypt.EncryptedMessage)),
	)(bb)
}

// Part of the builder chain, sets write deadline to the TCPTimeout global value
func ToggleWriteDeadline(writeDeadline bool) baseBuilderOption {
	return func(bb *baseBuilder) {
		bb.writeDeadlineActive = writeDeadline
	}
}

// Adds multiple identity-type objects that will be cloned to either send or receive messages.
// All protocol buffers you intend to send or receive should be registered with this method
// or the transmissions will fail
func AddMessageIdentities(messageIdents ...proto.Message) baseBuilderOption {
	return func(bb *baseBuilder) {
		for _, msgId := range messageIdents {
			addSingleIdentity(msgId, bb)
		}
	}
}

// AddMessageIdentity adds a single identity-type object that will be cloned to either send or receive messages.
// All protocol buffers you intend to send or receive should be registered with this method
// or the transmissions will fail
func AddMessageIdentity(messageIdent proto.Message) baseBuilderOption {
	return func(bb *baseBuilder) {
		addSingleIdentity(messageIdent, bb)
	}
}

// Route enables service or client to respond to receipt of a message with the given ExecutableRoute
//  NOTE: route must not be nil
func Route(messageIdent proto.Message, route ExecutableRoute) baseBuilderOption {
	return func(bb *baseBuilder) {
		addSingleIdentity(messageIdent, bb)
		addRoute(messageIdent, route, bb)
	}
}

func MaxConnections(max int) baseBuilderOption {
	return func(builder *baseBuilder) {
		builder.maxServerConnections = max
	}
}

func addSingleIdentity(messageIdent proto.Message, bb *baseBuilder) {
	t := getProtobufMessageName(messageIdent)
	i := int(hashString(t))
	bb.messageIdentMap[i] = messageIdent
	bb.inverseIdentMap[t] = i
}

func addRoute(messageIdent proto.Message, route ExecutableRoute, bb *baseBuilder) {
	if route == nil {
		panic(errors.New("ExecutableRoute cannot be nil"))
	}
	t := getProtobufMessageName(messageIdent)
	bb.routes[t] = route
}

// Part of the NanoBuilder chain, sets the number of messages that can be simultaneously placed on the send buffer
func SendBuffer(sendBuffer int) baseBuilderOption {
	return func(bb *baseBuilder) {
		bb.sendBuffer = sendBuffer
	}
}

// Part of the NanoBuilder chain, sets the number of messages that can be simultaneously placed on the
// receive buffer
func ReceiveBuffer(receiveBuffer int) baseBuilderOption {
	return func(bb *baseBuilder) {
		bb.receiveBuffer = receiveBuffer
	}
}

func PurgeConnectionsAfter(duration time.Duration) baseBuilderOption {
	return func(bb *baseBuilder) {
		bb.purgeConnections = duration
	}
}

func MaxIdleDuration(duration time.Duration) baseBuilderOption {
	return func(bb *baseBuilder) {
		bb.txRxIdleDuration = duration
	}
}

type clientDNSStrategy func(bool, *baseBuilder, <-chan *MDefinition)

func WithTimeout(duration time.Duration) clientDNSStrategy {
	return func(strict bool, bb *baseBuilder, definitionChannel <-chan *MDefinition) {
		select {
		case mdef, ok := <-definitionChannel:
			if ok {
				populateServiceFromMDef(bb.ns, mdef)
				if err := processMdef(bb, mdef, strict); err != nil {
					checkError(err)
				}
			} else {
				checkError(fmt.Errorf("DNS is closed"))
			}
		case <-time.After(duration):
			checkError(fmt.Errorf("client service discovery timeout after %v", duration.Truncate(time.Millisecond)))
		}
	}
}

func Default(strict bool, bb *baseBuilder, definitionChannel <-chan *MDefinition) {
	if mdef, ok := <-definitionChannel; ok {
		checkError(processMdef(bb, mdef, strict))
	} else {
		checkError(fmt.Errorf("DNS is closed"))
	}
}

func buildDNS(
	ctx context.Context,
	bb *baseBuilder,
	clientBuilder nanoClientFactory,
	strategy clientDNSStrategy,
) ClientDNSFactory {
	definitionChannel := startClientServiceDiscovery(ctx, bb.ns)
	return func(strictProtocols bool) (nan0 NanoServiceWrapper, err error) {
		defer recoverPanic(func(e error) {
			nan0 = nil
			err = e
		})
		strategy(strictProtocols, bb, definitionChannel)
		return clientBuilder(bb)
	}
}
