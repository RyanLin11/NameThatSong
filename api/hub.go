package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Room struct {
	roomCode      int
	roundNumber   int
	rounds        []Round
	participants  map[*Client]bool
	roundDuration time.Duration
}

type Round struct {
	Song        Song             `json:"song"`
	Correct     map[*Client]bool `json:"correct"`
	finishRound chan bool
	End         *time.Time `json:"exp"`
}

type Hub struct {
	createRoom  chan *CreateRoomRequest
	guess       chan *Guess
	join        chan *JoinRequest
	start       chan *Client
	leave       chan *Client
	clientRooms map[*Client]int
	rooms       map[int]*Room
	curRoom     int
}

type Guess struct {
	client *Client
	answer string
}

type JoinRequest struct {
	client *Client
	code   int
}

type CreateRoomRequest struct {
	client                 *Client
	numRounds              int
	roundDurationInSeconds int
}

var envFile, _ = godotenv.Read(".env")

func newHub() *Hub {
	return &Hub{
		createRoom:  make(chan *CreateRoomRequest),
		guess:       make(chan *Guess),
		join:        make(chan *JoinRequest),
		leave:       make(chan *Client),
		start:       make(chan *Client),
		clientRooms: make(map[*Client]int),
		rooms:       make(map[int]*Room),
		curRoom:     0,
	}
}

type SongResponse struct {
	Songs []Song `json:"results"`
}

type Song struct {
	Name       string `json:"trackName"`
	PreviewUrl string `json:"previewUrl"`
	ArtworkUrl string `json:"artworkUrl100"`
}

type RoomResponse struct {
	Code         int           `json:"code"`
	RoundNumber  int           `json:"round"`
	NumOfRounds  int           `json:"numRounds"`
	Participants []Participant `json:"participants"`
	Song         Song          `json:"song"`
	Exp          time.Time     `json:"exp"`
}

type Participant struct {
	Name          string `json:"name"`
	RoundsCorrect []int  `json:"roundsCorrect"`
}

func newRoom(roomCode int, numRounds int, durationInSeconds int, admin *Client) *Room {
	var responseObject SongResponse
	resp, err := http.Get(envFile["PREVIEW_API"])
	if err != nil {
		log.Fatalf("Failed to get songs: %v", err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&responseObject); err != nil {
		log.Fatalf("Failed to decode response: %v", err)
	}
	responseObject.Songs = responseObject.Songs[:numRounds]
	var rounds = make([]Round, len(responseObject.Songs))
	for i, song := range responseObject.Songs {
		rounds[i] = Round{song, make(map[*Client]bool), make(chan bool, 1), nil}
	}
	return &Room{
		roomCode:      roomCode,
		roundNumber:   -1,
		rounds:        rounds,
		participants:  map[*Client]bool{admin: true},
		roundDuration: time.Duration(durationInSeconds) * time.Second,
	}
}

func timer(room Room, complete chan int, quit chan bool) {
	log.Printf("timer started for round %v", room.roundNumber)
	select {
	case <-quit:
		log.Printf("timer cancelled for round %v", room.roundNumber)
	case <-time.After(room.roundDuration):
		log.Printf("timer's up")
		complete <- room.roomCode
	}
}

func broadcast(room Room) {
	log.Println("start broadcasting")
	roundsCorrect := make(map[*Client][]int)
	for participant := range room.participants {
		roundsCorrect[participant] = []int{}
	}
	for roundNo, round := range room.rounds {
		for correctGuesser := range round.Correct {
			roundsCorrect[correctGuesser] = append(roundsCorrect[correctGuesser], roundNo)
		}
	}
	participants := []Participant{}
	for client := range room.participants {
		participants = append(participants, Participant{client.name, roundsCorrect[client]})
	}
	var song Song
	var exp time.Time
	if room.roundNumber >= 0 && room.roundNumber < len(room.rounds) {
		song = room.rounds[room.roundNumber].Song
		exp = *room.rounds[room.roundNumber].End
	}
	res := RoomResponse{
		room.roomCode,
		room.roundNumber,
		len(room.rounds),
		participants,
		song,
		exp,
	}
	str, _ := json.Marshal(res)
	for client := range room.participants {
		log.Printf("sending value")
		client.send <- str
	}
}

func (h *Hub) advance(room *Room, timers chan int) {
	if room.roundNumber >= 0 {
		room.rounds[room.roundNumber].finishRound <- true
	}
	room.roundNumber++
	if room.roundNumber < len(room.rounds) {
		endTime := time.Now().Add(room.roundDuration).Local()
		room.rounds[room.roundNumber].End = &endTime
		go timer(*room, timers, room.rounds[room.roundNumber].finishRound)
	}
	broadcast(*room)
}

func (h *Hub) run() {
	timers := make(chan int)
	for {
		log.Println("iteration begin")
		select {
		case roomCode := <-timers:
			log.Println("Received timer message")
			room := h.rooms[roomCode]
			h.advance(room, timers)
			log.Println("timer message ends")
		case createRoomReq := <-h.createRoom:
			// TODO: check that the client is not still in a room
			log.Println("Received create message")
			h.clientRooms[createRoomReq.client] = h.curRoom
			h.rooms[h.curRoom] = newRoom(h.curRoom, createRoomReq.numRounds, createRoomReq.roundDurationInSeconds, createRoomReq.client)
			broadcast(*h.rooms[h.curRoom])
			h.curRoom++
			log.Println("create handle ends")
		case client := <-h.start:
			log.Println("Received start message")
			// TODO: check that the starter is the admin
			code, ok := h.clientRooms[client]
			if !ok {
				// error: user is not associated with a room
				log.Printf("error: user <%v> is not associated with a room", client.name)
				continue
			}
			room, ok := h.rooms[code]
			if !ok {
				// error: room does not exist
				log.Printf("error: user <%v> is not associated with a room", client.name)
				continue
			}
			if room.roundNumber != -1 {
				// error: room has already started or finished
				log.Printf("error: room %v has already started or has already finished", code)
				continue
			}
			h.advance(room, timers)
			log.Printf("info: user <%v> has started room %v", client.name, code)
		case guess := <-h.guess:
			log.Println("Received guess message")
			roomCode, ok := h.clientRooms[guess.client]
			if !ok {
				// error: user is not associated with a room
				log.Printf("error: user is not associated with a room")
				continue
			}
			room, ok := h.rooms[roomCode]
			if !ok {
				// error: room does not exist
				continue
			}
			if room.roundNumber < 0 {
				// error: room has not started yet
				continue
			}
			if room.roundNumber >= len(room.rounds) {
				// error: room has finished already
				continue
			}
			round := room.rounds[room.roundNumber]
			log.Println(room.rounds[0])
			log.Printf("%v guess for %v", guess.answer, round.Song.Name)
			changed := false
			if strings.EqualFold(strings.TrimSpace(guess.answer), strings.TrimSpace(round.Song.Name)) {
				round.Correct[guess.client] = true
				changed = true
			}
			log.Println(round.Correct)
			log.Println(len(room.participants))
			// the round finishes if all players successfully guess the song
			if len(round.Correct) == len(room.participants) {
				log.Println("Successful guess")
				h.advance(room, timers)
			} else if changed {
				broadcast(*room)
			}
			log.Println("guess handle ends")
		case req := <-h.join:
			log.Println("Received join message")
			room, ok := h.rooms[req.code]
			if !ok {
				// error: room does not exist
				continue
			}
			if room.roundNumber != -1 {
				// error: room has already started (optional)
				continue
			}
			log.Println(h.rooms)
			log.Println(req.code)
			_, ok = h.clientRooms[req.client]
			if ok {
				// error: client already belongs to different game
				continue
			}
			room.participants[req.client] = true
			h.clientRooms[req.client] = req.code
			broadcast(*room)
			log.Println("join handle ends")
		case client := <-h.leave:
			log.Println("Received leave message")
			// TODO: check that the user is associated with a room
			room_code := h.clientRooms[client]
			room := h.rooms[room_code]
			// remove the participant from the room
			delete(h.clientRooms, client)
			delete(room.participants, client)
			// delete the room if there is no more participants
			if len(room.participants) == 0 {
				if room.roundNumber < len(room.rounds) {
					room.rounds[room.roundNumber].finishRound <- true
				}
				delete(h.rooms, room_code)
			} else {
				broadcast(*h.rooms[room_code])
			}

			log.Println("leave handle ends")
		}
		log.Println("iteration complete")
	}
}
