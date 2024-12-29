import './App.css'
import useSongGuessRoom, { Status } from './services/Sockets'
import { HomePage } from './views/HomePage'
import { GuessingPage } from './views/GuessingPage'
import { WaitingPage } from './views/WaitingPage'
import { ResultsPage } from './views/ResultsPage'

function App() {
  const {
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
  } = useSongGuessRoom();

  return (
    <>
      { (status == Status.CONNECTING || status == Status.NOT_JOINED) && <HomePage setInputRoomNo={setInputRoomNo} setName={setName} createRoom={createRoom} /> }
      { (status == Status.JOINING || status == Status.NOT_STARTED) && <WaitingPage code={roomNo} participants={scores} startRoom={startRoom} /> }
      { (status == Status.IN_PROGRESS) && <GuessingPage roundNumber={roundNumber} participants={scores} song={song} setGuess={setGuess} exp={exp} /> }
      { (status == Status.FINISHED) && <ResultsPage code={roomNo} participants={scores} setInputRoomNo={setInputRoomNo} /> }
    </>
  )
}

export default App
