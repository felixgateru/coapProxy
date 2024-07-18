package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"

	gocoap "github.com/dustin/go-coap"
)

var (
	ProxyConn  *net.UDPConn
	ServerAddr *net.UDPAddr
	ClientDict map[string]*Connection = make(map[string]*Connection)
	conn       *Connection
	msgChan    = make(chan *net.UDPAddr, 1)
)

type Connection struct {
	ClientAddr *net.UDPAddr
	ServerConn *net.UDPConn
}

func setup(hostport string, port int) bool {
	saddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		slog.Error("Failed to resolve UDP address", slog.Any("err", err))
	}
	pudp, err := net.ListenUDP("udp", saddr)
	if err != nil {
		slog.Error("Failed to listen on UDP", slog.Any("err", err))
	}
	ProxyConn = pudp
	slog.Info("Listening on UDP", slog.Any("port", port))

	srvaddr, err := net.ResolveUDPAddr("udp", hostport)
	if err != nil {
		slog.Error("Failed to resolve UDP address", slog.Any("err", err))
	}

	ServerAddr = srvaddr
	serverConnection, err := net.DialUDP("udp", nil, srvaddr)
	if err != nil {
		slog.Error("Failed to listen on UDP", slog.Any("err", err))
	}
	conn = &Connection{
		ServerConn: serverConnection,
	}
	slog.Info("Server address", slog.Any("addr", srvaddr.String()))
	return true
}

func Down() {
	var buffer [1500]byte
	select {
	case msg := <-msgChan:
		for {
			n, err := conn.ServerConn.Read(buffer[0:])
			if err != nil {
				slog.Error("Failed to read from server", slog.Any("err", err))
			}

			_, err = ProxyConn.WriteToUDP(buffer[0:n], msg)
			if err != nil {
				slog.Error("Failed to write to client", slog.Any("err", err))
			}
		}

	}
}

func Up(conn *Connection, buffer []byte, n int) {
	msg, err := gocoap.ParseMessage(buffer[:n])
	if err != nil {
		slog.Error("Failed to parse CoAP message", slog.Any("err", err))
	}

	switch msg.Code {
	case gocoap.GET:
		handleGet()
	case gocoap.POST:
		handlePost()
	}

	_, err = conn.ServerConn.Write(buffer[0:n])
	if err != nil {
		slog.Error("Failed to write to server", slog.Any("err", err))
	}
}

func Proxy() {
	var buffer [1500]byte
	for {
		n, cliaddr, err := ProxyConn.ReadFromUDP(buffer[0:])
		if err != nil {
			slog.Error("Failed to read from client", slog.Any("err", err))
		}
		saddr := cliaddr.String()
		slog.Info("Received message from client", slog.Any("addr", saddr))
		go Down()
		msgChan <- cliaddr
		Up(conn, buffer[0:n], n)
	}
}

func main() {
	targetport := fmt.Sprintf("%s:%d", "localhost", 5683)
	proxyport := 5682
	if setup(targetport, proxyport) {
		Proxy()
	}
	os.Exit(0)
}

func handleGet() {
	slog.Info("GET request received")
}

func handlePost() {
	slog.Info("POST request received")
}
