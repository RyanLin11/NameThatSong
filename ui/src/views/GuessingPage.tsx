import {useEffect, useRef, useState} from 'react'
import {Participant, Song} from '../interfaces';


interface GuessingPageProps {
    roundNumber: number,
    participants: Array<Participant>,
    song: Song|undefined,
    exp: Date|undefined,
    setGuess: (guess: string) => void
}

export function GuessingPage(props: GuessingPageProps) {
    const [guess, setGuess] = useState("");
    const audioComponent = useRef<HTMLAudioElement>(null);
    const [timeRemaining, setTimeRemaining] = useState<number>();

    useEffect(() => {
        audioComponent.current?.load();
    }, [props.song?.url]);

    useEffect(() => {
        let intervalID: NodeJS.Timeout;
        let expiry = props.exp;
        if (expiry !== undefined) {
            intervalID = setInterval(() => {
                setTimeRemaining(Math.floor((expiry.getTime() - Date.now()) / 1000));
            }, 1000);
        }
        return () => clearTimeout(intervalID);
    }, [props.exp]);

    return (
        <div>
            <h2>Round {props.roundNumber} </h2>
            <h2>Time: {timeRemaining} seconds left</h2>
            <audio controls ref={audioComponent}>
                <source src={props.song?.url} type="audio/mpeg" />
            </audio>
            <h3> Participants </h3>
            <li>
                {props.participants?.map(participant => <ul>{participant.name}: {participant.roundsCorrect.length} {participant.roundsCorrect.includes(props.roundNumber) && <p>Correct</p>}</ul>)}
            </li>
            <div>
                <label htmlFor="guess">Guess</label>
                <input name="guess" type="text" value={guess} onChange={e => setGuess(e.target.value)} />
                <button type="submit" onClick={_ => props.setGuess(guess)}>Guess</button>
            </div>
        </div>
    )
}