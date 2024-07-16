package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	gocoap "github.com/dustin/go-coap"
)

var (
	ProxyConn  *net.UDPConn
	ServerAddr *net.UDPAddr
	ClientDict map[string]*Connection = make(map[string]*Connection)
	dmutex     *sync.Mutex            = new(sync.Mutex)
)

type Connection struct {
	ClientAddr *net.UDPAddr
	ServerConn *net.UDPConn
}

func NewConnection(srvAddr, cliAddr *net.UDPAddr) *Connection {
	conn := new(Connection)
	conn.ClientAddr = cliAddr
	srvudp, err := net.DialUDP("udp", nil, srvAddr)
	if err != nil {
		slog.Error("Failed to dial server", slog.Any("err", err))
	}
	conn.ServerConn = srvudp
	return conn
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
	slog.Info("Server address", slog.Any("addr", srvaddr.String()))
	return true
}

func Down(conn *Connection) {
	var buffer [1500]byte
	for {
		n, err := conn.ServerConn.Read(buffer[0:])
		if err != nil {
			slog.Error("Failed to read from server", slog.Any("err", err))
		}

		_, err = ProxyConn.WriteToUDP(buffer[0:n], conn.ClientAddr)
		if err != nil {
			slog.Error("Failed to write to client", slog.Any("err", err))
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
		dmutex.Lock()
		conn, found := ClientDict[saddr]
		if !found {
			conn = NewConnection(ServerAddr, cliaddr)
			if conn == nil {
				dmutex.Unlock()
				continue
			}
			ClientDict[saddr] = conn
			dmutex.Unlock()
			go Down(conn)
		} else {
			dmutex.Unlock()
		}
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
