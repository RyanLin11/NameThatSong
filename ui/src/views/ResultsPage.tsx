import {Participant} from '../interfaces'

interface ResultsPageProps {
    code: number|undefined
    participants: Array<Participant>
    setInputRoomNo: (code: undefined|number) => void
}

export function ResultsPage(props: ResultsPageProps) {
    return (
        <div>
            <h2>Results</h2>
            <ul>
                {props.participants.map(participant => 
                    <li>{participant.name}: {participant.roundsCorrect.length}</li>
                )}
            </ul>
            <button type="submit" onClick={() => props.setInputRoomNo(undefined)}> Exit </button>
        </div>
    )
}