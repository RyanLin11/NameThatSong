import {useState} from 'react'

interface HomePageProps {
    setInputRoomNo: (roomNo: number) => void
    setName: (name: string) => void
    createRoom: (artist: string, numOfRounds: number, duration: number) => void
}

export function HomePage(props: HomePageProps) {
    const [code, setCode] = useState(0);
    const [artist, setArtist] = useState("");
    const [duration, setDuration] = useState(15);
    const [name, setName] = useState("");
    const [numOfRounds, setNumOfRounds] = useState(1);
    return (
        <div>
            <div>
                <h2>Choose Your Name</h2>
                <label htmlFor="handle">Nickname</label>
                <input name="handle" type="text" value={name} onChange={e => setName(e.target.value)} />
                <button type="submit" onClick={_ => {props.setName(name); console.log('yo'); }}>Submit</button>
            </div>
            <div>
                <h2>Join Room</h2>
                <label htmlFor="code"> Room Code </label>
                <input name="code" type="number" value={code} onChange={e=>setCode(Number(e.target.value))}/>
                <button type="submit" onClick={() => props.setInputRoomNo(Number(code))}>Join</button>
            </div>
            <div>
                <h2>Create Room</h2>
                <label htmlFor="artist">Artist</label>
                <input name="artist" type="text" value={artist} onChange={e=>setArtist(e.target.value)}/>
                <label htmlFor="rounds">Number of Rounds</label>
                <input name="rounds" type="number" value={numOfRounds} onChange={e=>setNumOfRounds(Number(e.target.value))} />
                <label htmlFor="duration">Round Duration</label>
                <select name="duration" value={duration} onChange={e=>setDuration(Number(e.target.value))}>
                    <option value={15}>15 seconds</option>
                    <option value={20}>20 seconds</option>
                    <option value={25}>25 seconds</option>
                    <option value={30}>30 seconds</option>
                </select>
                <button type="submit" onClick={_ => props.createRoom(artist, numOfRounds, duration)}>Create Room</button>
            </div>
        </div>
    )
}