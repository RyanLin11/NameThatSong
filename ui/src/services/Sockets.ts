import {useState, useEffect, useRef} from 'react';
import {Participant, Round, Song} from '../interfaces';

export enum Status {
    CONNECTING,
    NOT_JOINED,
    JOINING,
    NOT_STARTED,
    IN_PROGRESS,
    FINISHED
};

export default function useSongGuessRoom() {
    const [roomNo, setRoomNo] = useState<number|undefined>();
    const [inputRoomNo, setInputRoomNo] = useState<number|undefined>();
    const [roundNumber, setRoundNumber] = useState(0);
    const [song, setSong] = useState<Song>();
    const [status, setStatus] = useState<Status>(Status.CONNECTING);
    const [guess, setGuess] = useState<string|undefined>();
    const [name, setName] = useState<string|undefined>();
    const [scores, setScores] = useState<Array<Participant>>([]);
    const [exp, setExp] = useState<Date|undefined>();

    var socketRef = useRef<WebSocket | null>(null);

    useEffect(() => {
        socketRef.current = new WebSocket('ws://localhost:8080/ws');
        socketRef.current.addEventListener("open", (_) => {
            console.log("WebSocket connection opened");
            setStatus(Status.NOT_JOINED);
        })
        socketRef.current.addEventListener("message", (event) => {
            let response = event.data;
            let round = JSON.parse(response) as Round;
            console.log(round);
            setRoundNumber(round.round);
            if (round.round == round.numRounds) {
                console.log('finished' + round.round + ' ' + round.numRounds)
                setScores(round.participants);
                setStatus(Status.FINISHED);
            } else if (round.round >= 0) {
                console.log('onto next round')
                setRoomNo(round.code);
                setInputRoomNo(round.code);
                setSong({title: round.song.trackName, url: round.song.previewUrl});
                setScores(round.participants);
                setExp(new Date(round.exp));
                setStatus(Status.IN_PROGRESS);
            } else {
                setInputRoomNo(round.code);
                setRoomNo(round.code);
                setStatus(Status.NOT_STARTED);
            }
        });
        socketRef.current.addEventListener("close", e => {
            console.log("WebSocket closed:", e.code, e.reason);
        })
        return () => {
            socketRef.current?.close();
        }
    }, []);

    useEffect(() => {
        console.log(roomNo);
        console.log(inputRoomNo);
        if (inputRoomNo !== undefined && roomNo === undefined) {
            socketRef.current?.send(JSON.stringify({
                type: 'join',
                code: inputRoomNo,
            }));
        } else if (inputRoomNo === undefined && roomNo !== undefined) {
            console.log('leaving');
            socketRef.current?.send(JSON.stringify({
                type: 'leave',
            }))
            setRoomNo(undefined);
            setRoundNumber(-1);
            setSong(undefined);
            setGuess(undefined);
            setScores([]);
            setStatus(Status.NOT_JOINED);
        }
    }, [inputRoomNo]);

    useEffect(() => {
        if (guess !== undefined) {
            socketRef.current?.send(JSON.stringify({
                type: 'guess',
                guess: guess,
            }));
        }
    }, [guess]);

    useEffect(() => {
        console.log('name update: '+name);
        if (name !== undefined) {
            socketRef.current?.send(JSON.stringify({
                type: 'name',
                name: name
            }));
        }
    }, [name]);

    function createRoom(artist: string, numOfRounds: number, duration: number) {
        socketRef.current?.send(JSON.stringify({
            type: 'create',
            numOfRounds: numOfRounds,
            roundDuration: duration,
            artist: artist
        }))
    }

    function startRoom() {
        socketRef.current?.send(JSON.stringify({
            type: 'start',
        }))
    }

    return {
        roomNo,
        roundNumber,
        song,
        status,
        scores,
        exp,
        setInputRoomNo,
        setGuess,
        setName,
        createRoom,
        startRoom
    };
}