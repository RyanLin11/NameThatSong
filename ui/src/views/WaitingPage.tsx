import {Participant} from '../interfaces'

interface WaitingPageProps {
    code: number|undefined
    participants: Array<Participant>
    startRoom: () => void
}

export function WaitingPage(props: WaitingPageProps) {
    return (
        <div>
            <h2>Room: {props.code}</h2>
            <ul>
                {props.participants.map(participant => 
                    <li>{participant.name}</li>
                )}
            </ul>
            <button type="submit" onClick={props.startRoom}> Start </button>
        </div>
    )
}