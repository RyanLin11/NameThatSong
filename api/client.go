package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	name string
}

type JoinRequestMessage struct {
	ReqType  string `json:"type"`
	RoomCode int    `json:"code"`
}

type CreateRequestMessage struct {
	ReqType                string `json:"type"`
	NumOfRounds            int    `json:"numOfRounds"`
	RoundDurationInSeconds int    `json:"roundDuration"`
}

type GuessMessage struct {
	ReqType string `json:"type"`
	Guess   string `json:"guess"`
}

type NameMessage struct {
	ReqType string `json:"type"`
	Name    string `json:"name"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// ONLY FOR DEVELOPMENT
		return true
		// return r.Header.Get("Origin") == "https://your-allowed-origin.com"
	},
}

func (c *Client) readPump() {
	defer func() {
		//c.hub.leave <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		var raw map[string]interface{}
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			} else {
				log.Printf("Another error occurred: %v", err)
			}
			break
		}
		if err := json.Unmarshal(message, &raw); err != nil {
			// TODO: let client know of invalid JSON
			continue
		}
		instr, ok := raw["type"]
		if !ok {
			// TODO: let client know of malformed message (missing "type")
			log.Printf("missing type from message")
			continue
		}
		switch instr {
		case "name":
			var req NameMessage
			err := json.Unmarshal(message, &req)
			if err != nil {
				// TODO: let client know that type name message has incorrect schema
				log.Printf("name message is not of correct format")
				continue
			}
			c.name = req.Name
		case "join":
			var req JoinRequestMessage
			err := json.Unmarshal(message, &req)
			if err != nil {
				// TODO: let client know that type join message has incorrect schema
				log.Printf("join message is not of correct format")
				continue
			}
			c.hub.join <- &JoinRequest{c, req.RoomCode}
		case "create":
			log.Println("received create message")
			var req CreateRequestMessage
			err := json.Unmarshal(message, &req)
			if err != nil {
				// TODO: let client know that type create message has incorrect schema
				log.Printf("create message is not of correct format")
				continue
			}
			c.hub.createRoom <- &CreateRoomRequest{c, req.NumOfRounds, req.RoundDurationInSeconds}
		case "start":
			log.Println("start message received by client")
			c.hub.start <- c
			log.Println("start message processed")
		case "guess":
			var req GuessMessage
			err := json.Unmarshal(message, &req)
			if err != nil {
				// TODO: let client know that type guess message has incorrect schema
				log.Printf("guess message is not of correct format")
				continue
			}
			c.hub.guess <- &Guess{c, req.Guess}
		case "leave":
			c.hub.leave <- c
		}
	}
	log.Println("Finished reading")
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				log.Println("hub closed the channel")
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Println("error with next writer")
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				log.Println("error closing writer")
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
