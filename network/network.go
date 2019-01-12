package network

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"reflect"
	"time"

	errs "github.com/go-errors/errors"
	uuid "github.com/satori/go.uuid"
)

func delimiter() byte {
	return '\n'
}

func MustMarshal(i interface{}) []byte {
	bytes, err := json.Marshal(i)
	if err != nil {
		panic(errs.New(err))
	}
	return bytes
}

func MustUnmarshal(bytes []byte, ptr interface{}) {
	err := json.Unmarshal(bytes, ptr)
	if err != nil {
		panic(errs.New(err))
	}
}

func MakeUUID() uuid.UUID {
	return uuid.Must(uuid.NewV4())
}

type Message struct {
	ClientID    uuid.UUID
	Content     json.RawMessage
	ContentType string
}

func (message Message) String() string {
	return `{"Message": ` + string(MustMarshal(message)) + `}`
}

func MakeMessage(clientID uuid.UUID, contents interface{}) Message {
	bytes, err := json.Marshal(contents)
	if err != nil {
		panic(errs.New(err))
	}

	contentType := reflect.TypeOf(contents).String()

	return Message{
		ClientID:    clientID,
		Content:     bytes,
		ContentType: contentType,
	}
}

type Tunnel struct {
	ID       uuid.UUID
	Incoming chan Message
	Outgoing chan Message
	Closed   chan struct{}
}

type ConnInfo struct {
	host string
	id   uuid.UUID
	port int

	pemPath  string
	certPath string
}

func MakeConnInfo(host string, port int, id uuid.UUID, pemPath, certPath string) ConnInfo {
	return ConnInfo{
		host: host,
		id:   id,
		port: port,

		pemPath:  pemPath,
		certPath: certPath,
	}
}

type Server struct {
	info ConnInfo
}

func NewServer(info ConnInfo) *Server {
	return &Server{
		info: info,
	}
}

func (server *Server) Serve() (func() error, chan Tunnel, error) {
	emptyClose := func() error { return nil }
	tunnels := make(chan Tunnel, 100)

	if server.info.pemPath == "" || server.info.certPath == "" {
		return emptyClose, tunnels, errs.New("invalid pem/cert path")
	}

	cert, err := tls.LoadX509KeyPair(server.info.pemPath, server.info.certPath)
	if err != nil {
		return emptyClose, tunnels, errs.New(err)
	}

	config := tls.Config{Certificates: []tls.Certificate{cert}}

	listener, err := tls.Listen(
		"tcp",
		fmt.Sprintf("%s:%d", server.info.host, server.info.port),
		&config,
	)

	if err != nil {
		return emptyClose, tunnels, errs.New(err)
	}

	go func(l net.Listener) {
		for {
			conn, err := listener.Accept()
			if err != nil {
				panic(errs.New(err))
			}

			clientID, err := server.handshake(conn)
			if err != nil {
				panic(errs.New(err))
			}

			incoming := make(chan Message, 100)
			outgoing := make(chan Message, 100)
			closed := make(chan struct{}, 1)

			tunnel := Tunnel{
				ID:       clientID,
				Incoming: incoming,
				Outgoing: outgoing,
				Closed:   closed,
			}

			tunnels <- tunnel

			go server.receive(conn, tunnel)
			go server.send(conn, tunnel)
		}
	}(listener)

	return listener.Close, tunnels, nil
}

func (s Server) handshake(conn net.Conn) (uuid.UUID, error) {
	var clientID uuid.UUID

	buff := bufio.NewReader(conn)
	rawID, err := buff.ReadBytes(delimiter())
	if err != nil {
		return clientID, errs.New(err)
	}

	clientID, err = uuid.FromBytes(rawID[:len(rawID)-1])
	if err != nil {
		return clientID, errs.New(err)
	}

	if _, err = conn.Write(s.info.id.Bytes()); err != nil {
		return clientID, errs.New(err)
	}

	if _, err = conn.Write([]byte{delimiter()}); err != nil {
		return clientID, errs.New(err)
	}

	return clientID, nil
}

func (s Server) receive(conn net.Conn, tunnel Tunnel) {
	var rawMessage []byte
	var err error

	buff := bufio.NewReader(conn)

	for {
		rawMessage, err = buff.ReadBytes(delimiter())
		if err != nil {
			if _, ok := err.(net.Error); ok {
				break
			}
			tunnel.Closed <- struct{}{}
			//panic(errs.New(err))
			fmt.Println(errs.New(err))
			return
		}
		rawMessage = rawMessage[:len(rawMessage)-1]

		message := Message{}
		err = json.Unmarshal(rawMessage, &message)
		if err != nil {
			panic(errs.New(err))
			tunnel.Closed <- struct{}{}
		}

		tunnel.Incoming <- message
		conn.SetDeadline(time.Now().Add(2 * time.Minute))
	}
}

func (s Server) send(conn net.Conn, tunnel Tunnel) {
	for message := range tunnel.Outgoing {
		rawMessage, err := json.Marshal(message)
		if err != nil {
			tunnel.Closed <- struct{}{}
			panic(errs.New(err))
		}

		if _, err = conn.Write(rawMessage); err != nil {
			tunnel.Closed <- struct{}{}
			panic(errs.New(err))
		}

		if _, err = conn.Write([]byte{delimiter()}); err != nil {
			tunnel.Closed <- struct{}{}
			panic(errs.New(err))
		}

		conn.SetDeadline(time.Now().Add(2 * time.Minute))
	}
}

type Client struct {
	info ConnInfo
}

func NewClient(info ConnInfo) *Client {
	return &Client{
		info: info,
	}
}

func (client *Client) Dial() (func() error, Tunnel, error) {
	incoming := make(chan Message, 100)
	outgoing := make(chan Message, 100)
	closed := make(chan struct{}, 1)

	tunnel := Tunnel{
		Incoming: incoming,
		Outgoing: outgoing,
		Closed:   closed,
	}

	caPool := x509.NewCertPool()
	serverCert, err := ioutil.ReadFile(client.info.pemPath)
	if err != nil {
		return nil, tunnel, errs.New(err)
	}

	caPool.AppendCertsFromPEM(serverCert)
	config := tls.Config{
		RootCAs: caPool,
	}

	conn, err := tls.Dial(
		"tcp",
		fmt.Sprintf("%s:%d", client.info.host, client.info.port),
		&config,
	)
	if err != nil {
		return nil, tunnel, errs.New(err)
	}

	serverID, err := client.handshake(conn)
	if err != nil {
		return nil, tunnel, errs.New(err)
	}

	tunnel.ID = serverID

	go client.send(conn, tunnel)
	go client.receive(conn, tunnel)
	return conn.Close, tunnel, nil
}

func (client Client) handshake(conn net.Conn) (uuid.UUID, error) {
	var serverID uuid.UUID

	if _, err := conn.Write(client.info.id.Bytes()); err != nil {
		return serverID, errs.New(err)
	}

	if _, err := conn.Write([]byte{delimiter()}); err != nil {
		return serverID, errs.New(err)
	}

	buff := bufio.NewReader(conn)
	rawID, err := buff.ReadBytes(delimiter())
	if err != nil {
		return serverID, errs.New(err)
	}

	serverID, err = uuid.FromBytes(rawID[:len(rawID)-1])
	if err != nil {
		return serverID, errs.New(err)
	}

	return serverID, nil
}

func (client Client) send(c net.Conn, tunnel Tunnel) {
	for message := range tunnel.Outgoing {
		rawMessage, err := json.Marshal(message)
		if err != nil {
			panic(errs.New(err))
			fmt.Println(errs.New(err))
			return
		}

		if _, err = c.Write(rawMessage); err != nil {
			//panic(errs.New(err))
			fmt.Println(errs.New(err))
			return
		}

		if _, err = c.Write([]byte{delimiter()}); err != nil {
			//panic(errs.New(err))
			fmt.Println(errs.New(err))
			return
		}

		c.SetDeadline(time.Now().Add(2 * time.Minute))
	}
}

func (client Client) receive(c net.Conn, tunnel Tunnel) {
	buff := bufio.NewReader(c)
	for {
		rawMessage, err := buff.ReadBytes(delimiter())
		if err != nil {
			//panic(errs.New(err))
			fmt.Println(errs.New(err))
			return
		}
		c.SetDeadline(time.Now().Add(2 * time.Minute))
		rawMessage = rawMessage[:len(rawMessage)-1]

		message := Message{}
		if err = json.Unmarshal(rawMessage, &message); err != nil {
			panic(errs.New(err))
		}

		tunnel.Incoming <- message
	}
}
