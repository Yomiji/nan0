module github.com/yomiji/nan0

go 1.12

require (
	github.com/golang/protobuf v1.3.2
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/yomiji/slog v1.1.2
	github.com/yomiji/websocket v1.4.1
)

replace github.com/yomiji/websocket v1.4.1 => github.com/gorilla/websocket v1.4.1
