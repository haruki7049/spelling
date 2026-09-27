package match

// Result is the outcome of a match.
type Result int

const (
	Ongoing Result = iota
	Player0Wins
	Player1Wins
	Draw
)

// TimeLimitTicks is the match length (tentative: 3 minutes, see issue #15).
const TimeLimitTicks = 3 * 60 * TicksPerSecond

func (r Result) String() string {
	switch r {
	case Player0Wins:
		return "player 0 wins"
	case Player1Wins:
		return "player 1 wins"
	case Draw:
		return "draw"
	}
	return "ongoing"
}

// Result returns the outcome so far.
func (m *Match) Result() Result {
	return m.result
}

// judge decides the match at the end of a tick (see issue #15):
//   - HP at 0 loses; both at 0 in the same tick is a draw.
//   - Both players depleted is a draw; one depleted player keeps playing.
//   - At the time limit, the higher remaining HP ratio wins; equal is a draw.
func (m *Match) judge() Result {
	a, b := m.World.Bodies[0], m.World.Bodies[1]
	switch {
	case a.HP <= 0 && b.HP <= 0:
		return Draw
	case b.HP <= 0:
		return Player0Wins
	case a.HP <= 0:
		return Player1Wins
	case a.Depleted && b.Depleted:
		return Draw
	case m.Tick < TimeLimitTicks:
		return Ongoing
	case a.HP > b.HP: // both have the same max HP, so HP compares ratios
		return Player0Wins
	case b.HP > a.HP:
		return Player1Wins
	}
	return Draw
}
