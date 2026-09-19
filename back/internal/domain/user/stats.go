package user

// StatsDelta is a change to the progression aggregate. Every field is a
// difference, not a value, so two concurrent completions add up instead of
// overwriting each other.
type StatsDelta struct {
	XP                int64
	TasksCreated      int32
	TasksCompleted    int32
	SubtasksCompleted int32
	LinksCreated      int32
}

// Empty reports a delta that would change nothing.
func (d *StatsDelta) Empty() bool {
	return d.XP == 0 && d.TasksCreated == 0 && d.TasksCompleted == 0 &&
		d.SubtasksCompleted == 0 && d.LinksCreated == 0
}

// StreakChange is what marking a day did.
//
// Extended is false when the day was already counted, which is the common
// case: a user opens the app many times a day and the streak moves once.
type StreakChange struct {
	Extended bool
	Current  int32
	Longest  int32
}
