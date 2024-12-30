package main

import (
	"encoding/json"
	"log"
	"log/slog"
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
		slog.Error("Failed to get songs", slog.Any("error", err))
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&responseObject); err != nil {
		slog.Error("Failed to decode response", slog.Any("response", resp.Body), slog.Any("error", err))
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
	slog.Info("timer started", slog.Int("room", room.roomCode), slog.Int("round", room.roundNumber))
	select {
	case <-quit:
		slog.Info("timer cancelled", slog.Int("room", room.roomCode), slog.Int("round", room.roundNumber))
	case <-time.After(time.Until(*room.rounds[room.roundNumber].End)):
		slog.Info("timer completed", slog.Int("room", room.roomCode), slog.Int("round", room.roundNumber))
		complete <- room.roomCode
	}
}

func broadcast(room Room) {
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
		select {
		case roomCode := <-timers:
			slog.Info("Time's up for room", slog.Int("room", roomCode))
			h.advance(h.rooms[roomCode], timers)
		case createRoomReq := <-h.createRoom:
			slog.Info("Received create message", slog.String("user", createRoomReq.client.name))
			currentRoomCode, ok := h.clientRooms[createRoomReq.client]
			if ok {
				slog.Info("user is still in another game", slog.String("user", createRoomReq.client.name), slog.Int("other room", currentRoomCode))
				continue
			}
			_, ok = h.rooms[h.curRoom]
			if ok {
				slog.Error("room already exists with code", slog.Int("code", h.curRoom))
				continue
			}
			h.clientRooms[createRoomReq.client] = h.curRoom
			h.rooms[h.curRoom] = newRoom(h.curRoom, createRoomReq.numRounds, createRoomReq.roundDurationInSeconds, createRoomReq.client)
			broadcast(*h.rooms[h.curRoom])
			h.curRoom++
		case client := <-h.start:
			slog.Info("Received start message", slog.String("user", client.name))
			code, ok := h.clientRooms[client]
			if !ok {
				slog.Info("user is not associated with a room", slog.String("user", client.name))
				continue
			}
			room, ok := h.rooms[code]
			if !ok {
				slog.Info("room does not exist", slog.Int("room", code))
				continue
			}
			if room.roundNumber != -1 {
				slog.Info("room has already started or has already finished", slog.Int("room", code))
				continue
			}
			h.advance(room, timers)
			slog.Info("user has started room", slog.String("user", client.name), slog.Int("room", code))
		case guess := <-h.guess:
			slog.Info("Guess received", slog.String("user", guess.client.name), slog.String("guess", guess.answer))
			roomCode, ok := h.clientRooms[guess.client]
			if !ok {
				slog.Info("user is not associated with a room", slog.String("user", guess.client.name))
				continue
			}
			room, ok := h.rooms[roomCode]
			if !ok {
				slog.Info("room does not exist", slog.String("user", guess.client.name), slog.Int("room", roomCode))
				continue
			}
			if room.roundNumber < 0 {
				slog.Info("room has not started yet", slog.String("user", guess.client.name), slog.Int("room", roomCode))
				continue
			}
			if room.roundNumber >= len(room.rounds) {
				slog.Info("room has finished already", slog.String("user", guess.client.name), slog.Int("room", roomCode))
				continue
			}
			round := room.rounds[room.roundNumber]
			if round.Correct[guess.client] {
				slog.Info("Client submitted again after already receiving right answer, no change will be made and no message will be broadcast", slog.String("user", guess.client.name))
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(guess.answer), strings.TrimSpace(round.Song.Name)) {
				slog.Info("Wrong guess made by client, no change to state", slog.String("user", guess.client.name), slog.String("guess", guess.answer), slog.String("correct answer", round.Song.Name))
				continue
			}
			round.Correct[guess.client] = true
			// the round finishes if all players successfully guess the song
			if len(round.Correct) == len(room.participants) {
				slog.Info("User guessed right, moving on to next round")
				h.advance(room, timers)
			} else {
				slog.Info("User guessed right, waiting for others")
				broadcast(*room)
			}
		case req := <-h.join:
			slog.Info("Received join message", slog.String("user", req.client.name), slog.Int("room", req.code))
			room, ok := h.rooms[req.code]
			if !ok {
				slog.Info("room does not exist", slog.Int("room", req.code))
				continue
			}
			if room.roundNumber != -1 {
				slog.Info("room has already started", slog.Int("room", req.code))
				continue
			}
			otherRoomCode, ok := h.clientRooms[req.client]
			if ok {
				slog.Info("user already belongs to different game", slog.String("user", req.client.name), slog.Int("other room", otherRoomCode))
				continue
			}
			room.participants[req.client] = true
			h.clientRooms[req.client] = req.code
			broadcast(*room)
		case client := <-h.leave:
			log.Println("Received leave message", slog.String("user", client.name))
			roomCode, ok := h.clientRooms[client]
			if !ok {
				slog.Info("user was not part of any room, no cleanup required", slog.String("user", client.name))
				continue
			}
			delete(h.clientRooms, client)
			room, ok := h.rooms[roomCode]
			if !ok {
				slog.Info("user was part of a room that didn't exist, no cleanup will be performed", slog.String("user", client.name), slog.Int("room", roomCode))
				continue
			}
			delete(room.participants, client)
			// delete the room if there is no more participants
			if len(room.participants) == 0 {
				if room.roundNumber < len(room.rounds) {
					room.rounds[room.roundNumber].finishRound <- true
				}
				delete(h.rooms, roomCode)
			} else {
				broadcast(*h.rooms[roomCode])
			}
		}
	}
}
