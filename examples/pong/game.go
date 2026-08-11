package main

// The rules, which run on one side only.
//
// Somebody has to be right about where the ball is. If both sides simulated it
// they would drift apart within seconds — two machines cannot agree on physics
// over a lossy link without a lot of machinery. So the host simulates and the
// joiner draws what it is told, which is the oldest and simplest answer.
type game struct {
	state State

	ballDX, ballDY int
	paddle         [2]uint16 // 0 is the host's, 1 is the joiner's
}

const (
	paddleHeight = 160
	paddleSpeed  = 26
	ballSpeed    = 12
	winningScore = 11
)

func newGame() *game {
	game := &game{}
	game.paddle[0] = fieldHeight / 2
	game.paddle[1] = fieldHeight / 2
	game.serve(1)
	return game
}

// serve puts the ball back in the middle, heading towards whoever was scored
// against. Direction is a plain sign, not randomness: two machines cannot agree
// on a coin toss without sending it, and this needs no agreement at all.
func (g *game) serve(direction int) {
	g.state.BallX = fieldWidth / 2
	g.state.BallY = fieldHeight / 2
	g.ballDX = ballSpeed * direction
	g.ballDY = ballSpeed / 2
	g.state.Serving = true
}

// tick advances the world by one frame.
func (g *game) tick() {
	g.state.LeftY = g.paddle[0]
	g.state.RightY = g.paddle[1]

	if g.finished() {
		return
	}
	g.state.Serving = false

	x := int(g.state.BallX) + g.ballDX
	y := int(g.state.BallY) + g.ballDY

	// The top and bottom walls.
	if y <= 0 {
		y, g.ballDY = 0, -g.ballDY
	}
	if y >= fieldHeight {
		y, g.ballDY = fieldHeight, -g.ballDY
	}

	switch {
	case x <= paddleInset:
		if hits(y, g.paddle[0]) {
			x, g.ballDX = paddleInset, -g.ballDX
			g.ballDY += lean(y, g.paddle[0])
		} else {
			g.state.RightScore++
			g.serve(1)
			return
		}
	case x >= fieldWidth-paddleInset:
		if hits(y, g.paddle[1]) {
			x, g.ballDX = fieldWidth-paddleInset, -g.ballDX
			g.ballDY += lean(y, g.paddle[1])
		} else {
			g.state.LeftScore++
			g.serve(-1)
			return
		}
	}

	g.state.BallX = uint16(x)
	g.state.BallY = uint16(y)
}

// paddleInset is how far the paddles sit from their walls.
const paddleInset = 30

func hits(ballY int, paddleY uint16) bool {
	top := int(paddleY) - paddleHeight/2
	return ballY >= top && ballY <= top+paddleHeight
}

// lean makes the ball come off a paddle at an angle depending on where it hit,
// which is the whole of what makes this a game rather than a demo.
func lean(ballY int, paddleY uint16) int {
	offset := ballY - int(paddleY)
	return offset / 12
}

func (g *game) move(player int, delta int) {
	position := int(g.paddle[player]) + delta
	switch {
	case position < 0:
		position = 0
	case position > fieldHeight:
		position = fieldHeight
	}
	g.paddle[player] = uint16(position)
}

// reset starts the match over: scores to nothing, ball in the middle. Only the
// host runs it, and the guest learns about it from the next state like it
// learns about everything else.
func (g *game) reset() {
	g.state.LeftScore = 0
	g.state.RightScore = 0
	g.serve(1)
}

func (g *game) finished() bool {
	return g.state.LeftScore >= winningScore || g.state.RightScore >= winningScore
}
