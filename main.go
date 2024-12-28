package main

import (
	"fmt"
	"net"
	"strings"
)

type Server struct {
	clients    chan Client
	broadcast  chan Message
	register   chan Client
	unregister chan Client
}

type Client struct {
	Conn     net.Conn
	Nickname string
}

type Message struct {
	Client Client
	Msg    string
}

func NewServer() *Server {
	return &Server{
		clients:    make(chan Client),
		broadcast:  make(chan Message),
		register:   make(chan Client),
		unregister: make(chan Client),
	}
}

func NewClient(conn net.Conn, nickname string) *Client {
	return &Client{
		Conn:     conn,
		Nickname: nickname,
	}
}

func (c *Client) Write(data []byte) {
	c.Conn.Write(data)
}

func newMessage(client Client, msg string) *Message {
	return &Message{
		Client: client,
		Msg:    msg,
	}
}

func (s *Server) Run() {
	conns := make(map[Client]bool)
	for {
		select {
		case client := <-s.register:
			conns[client] = true
		case client := <-s.unregister:
			if _, ok := conns[client]; ok {
				delete(conns, client)
				client.Conn.Close()
			}
		case msg := <-s.broadcast:
			for client := range conns {
				if client.Conn == msg.Client.Conn {
					continue
				}
				client.Write([]byte(msg.Msg))
			}
		}
	}
}

func main() {
	listener, err := net.Listen("tcp", ":31337")
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}

	server := NewServer()
	go server.Run()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			continue
		}
		client := NewClient(conn, "Anonymous")
		server.register <- *client
		server.broadcast <- *newMessage(*client, fmt.Sprintf("%s connected\n", client.Nickname))
		go handleRequest(client, server)
	}
}

func handleRequest(client *Client, server *Server) {
	client.Write([]byte("Hello from server\n"))
	client.Write([]byte("Type /exit to quit\n"))
	client.Write([]byte("Type /nick <name> to change your name\n"))

	defer func() {
		server.unregister <- *client
	}()

	for {
		buf := make([]byte, 1024)
		reqLen, err := client.Conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading:", err.Error())
			return
		}

		message := strings.TrimSpace(string(buf[:reqLen]))
		if message == "/exit" {
			server.broadcast <- *newMessage(*client, fmt.Sprintf("%s disconnected\n", client.Nickname))
			client.Write([]byte("Goodbye!\n"))
			return
		}

		if strings.HasPrefix(message, "/nick") {
			parts := strings.Split(message, " ")
			if len(parts) < 2 {
				client.Write([]byte("Usage: /nick <name>\n"))
				continue
			}
			oldNick := client.Nickname
			client.Nickname = parts[1]
			client.Write([]byte(fmt.Sprintf("Changed nickname from %s to %s\n", oldNick, client.Nickname)))
			server.broadcast <- *newMessage(*client, fmt.Sprintf("%s changed nickname to %s\n", oldNick, client.Nickname))
			continue
		}

		newMsg := *newMessage(*client, fmt.Sprintf("%s: %s\n", client.Nickname, message))
		server.broadcast <- newMsg
	}
}
