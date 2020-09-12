package nan0_tests

import (
	"os"
	"testing"
	"time"

	"github.com/yomiji/nan0/v2"
	"github.com/yomiji/slog"
	"google.golang.org/protobuf/proto"
)

const testTimeout = 15 * time.Second
const nsDefaultPort int32 = 2324
const wsDefaultPort int32 = 8080

var wsServer nan0.Server

func TestMain(m *testing.M) {
	slog.ToggleLogging(true, true, true, true)
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
