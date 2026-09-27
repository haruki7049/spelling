package match

import "testing"

func TestMatchStartsOngoing(t *testing.T) {
	m := newMatch(t)
	m.Step()
	if r := m.Result(); r != Ongoing {
		t.Errorf("result = %v, want Ongoing", r)
	}
}

func TestZeroHPLoses(t *testing.T) {
	for loser, want := range []Result{Player1Wins, Player0Wins} {
		m := newMatch(t)
		m.World.Bodies[loser].HP = 0
		m.Step()
		if r := m.Result(); r != want {
			t.Errorf("player %d at 0 HP: result = %v, want %v", loser, r, want)
		}
	}
}

func TestBothZeroHPIsADraw(t *testing.T) {
	m := newMatch(t)
	m.World.Bodies[0].HP, m.World.Bodies[1].HP = 0, 0
	m.Step()
	if r := m.Result(); r != Draw {
		t.Errorf("result = %v, want Draw", r)
	}
}

func TestOneDepletedPlayerKeepsPlaying(t *testing.T) {
	m := newMatch(t)
	m.World.Bodies[0].Depleted = true
	m.Step()
	if r := m.Result(); r != Ongoing {
		t.Errorf("result = %v, want Ongoing (depletion alone does not lose)", r)
	}
}

func TestBothDepletedIsADraw(t *testing.T) {
	m := newMatch(t)
	m.World.Bodies[0].Depleted, m.World.Bodies[1].Depleted = true, true
	m.Step()
	if r := m.Result(); r != Draw {
		t.Errorf("result = %v, want Draw", r)
	}
}

func TestTimeLimit(t *testing.T) {
	for _, tt := range []struct {
		hp0, hp1 int32
		want     Result
	}{
		{80, 50, Player0Wins},
		{30, 60, Player1Wins},
		{70, 70, Draw},
	} {
		m := newMatch(t)
		m.Tick = TimeLimitTicks - 1
		m.World.Bodies[0].HP, m.World.Bodies[1].HP = tt.hp0, tt.hp1
		m.Step()
		if r := m.Result(); r != tt.want {
			t.Errorf("HP %d vs %d at the time limit: result = %v, want %v", tt.hp0, tt.hp1, r, tt.want)
		}
	}
}

func TestNotOverBeforeTheTimeLimit(t *testing.T) {
	m := newMatch(t)
	m.Tick = TimeLimitTicks - 2
	m.World.Bodies[0].HP = 50
	m.Step()
	if r := m.Result(); r != Ongoing {
		t.Errorf("result = %v one tick before the limit, want Ongoing", r)
	}
}

func TestMatchStopsWhenOver(t *testing.T) {
	m := newMatch(t)
	m.World.Bodies[1].HP = 0
	m.Step()
	tick, x := m.Tick, m.World.Bodies[0].X
	m.World.Bodies[0].VX = 8 << 16
	m.Step()
	if m.Tick != tick || m.World.Bodies[0].X != x {
		t.Error("the match kept running after it was decided")
	}
}

func TestResultString(t *testing.T) {
	for r, want := range map[Result]string{
		Ongoing: "ongoing", Player0Wins: "player 0 wins", Player1Wins: "player 1 wins", Draw: "draw",
	} {
		if r.String() != want {
			t.Errorf("%d.String() = %q, want %q", r, r.String(), want)
		}
	}
}
