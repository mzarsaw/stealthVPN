module stealthvpn/client/linux

go 1.23.0

toolchain go1.24.3

require (
	github.com/gorilla/websocket v1.5.1
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	stealthvpn/pkg/protocol v0.0.0
)

require (
	golang.org/x/crypto v0.39.0 // indirect
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
)

replace stealthvpn/pkg/protocol => ../../pkg/protocol
