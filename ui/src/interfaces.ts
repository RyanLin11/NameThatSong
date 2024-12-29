export interface Participant {
    name: string
    roundsCorrect: Array<number>
}

export interface Song {
    title: string,
    url: string
}

export interface Round {
    code: number
    round: number
    numRounds: number
    participants: Array<Participant>
    song: {trackName: string, previewUrl: string}
    exp: string
}