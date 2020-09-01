package nan0_tests

import (
	"os"
	"testing"

	"github.com/yomiji/nan0/v2"
	"github.com/yomiji/slog"
	"google.golang.org/protobuf/proto"
)

var nsDefaultPort int32 = 2324
var wsDefaultPort int32 = 8080

var wsServer nan0.Server

func StartTestServerThread(wsServer nan0.Server) {
	go func() {
		if conn, ok := <-wsServer.GetConnections(); ok {
			select {
			case msg, ok := <-conn.GetReceiver():
				if ok {
					conn.GetSender() <- msg
				}
			}
		}
	}()
}

func TestMain(m *testing.M) {
	slog.ToggleLogging(true, true, true, false)
	//slog.ToggleLineNumberPrinting(true, true, true, false)
	//slog.FilterSource("nan0Util.go")
	//slog.FilterSource("nan0.go")
	//slog.FilterSource("encryption.go")
	//slog.FilterSource("nanoBuilder.go")
	os.Exit(m.Run())
}

type TestRoute struct{}

func (TestRoute) Execute(msg proto.Message, sender chan<- interface{}) {
	sender <- msg
}
