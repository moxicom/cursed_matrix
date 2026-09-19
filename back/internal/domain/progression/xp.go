package progression

import (
	"math"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// Config is the versioned XP and level configuration.
type Config struct {
	Version int

	TaskXP map[shared.Quadrant]int32

	SubtaskMultiplier float64

	LevelFactor float64
}

func DefaultConfig() Config {
	return Config{
		Version: 1,
		TaskXP: map[shared.Quadrant]int32{
			shared.QuadrantImportantUrgent:      50,
			shared.QuadrantImportantNotUrgent:   35,
			shared.QuadrantNotImportantUrgent:   20,
			shared.QuadrantNotImportantNotUrgnt: 10,
		},
		SubtaskMultiplier: 0.35,
		LevelFactor:       45,
	}
}

// Reward returns the XP for completing a task in the given effective quadrant.
func (c *Config) Reward(quadrant shared.Quadrant, isSubtask bool) (int32, error) {
	base, ok := c.TaskXP[quadrant]
	if !ok {
		return 0, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "quadrant", "value": string(quadrant)})
	}
	if !isSubtask {
		return base, nil
	}
	return int32(math.Round(float64(base) * c.SubtaskMultiplier)), nil
}

func (c *Config) XPForLevel(level int32) int64 {
	if level <= 1 {
		return 0
	}
	n := float64(level - 1)
	return int64(math.Round(c.LevelFactor * n * n))
}

func (c *Config) LevelForXP(xp int64) int32 {
	if xp <= 0 {
		return 1
	}
	level := int32(math.Floor(math.Sqrt(float64(xp)/c.LevelFactor))) + 1
	if level < 1 {
		return 1
	}
	return level
}

type Progress struct {
	Level     int32
	Into      int64
	Span      int64
	Remaining int64
}

func (c *Config) Progress(xp int64) Progress {
	level := c.LevelForXP(xp)
	current := c.XPForLevel(level)
	next := c.XPForLevel(level + 1)
	span := max(next-current, 1)
	return Progress{Level: level, Into: xp - current, Span: span, Remaining: next - xp}
}
