package shared

import "database/sql/driver"

type Quadrant string

const (
	QuadrantImportantUrgent      Quadrant = "IMPORTANT_URGENT"
	QuadrantImportantNotUrgent   Quadrant = "IMPORTANT_NOT_URGENT"
	QuadrantNotImportantUrgent   Quadrant = "NOT_IMPORTANT_URGENT"
	QuadrantNotImportantNotUrgnt Quadrant = "NOT_IMPORTANT_NOT_URGENT"
)

var quadrants = []Quadrant{
	QuadrantImportantUrgent,
	QuadrantImportantNotUrgent,
	QuadrantNotImportantUrgent,
	QuadrantNotImportantNotUrgnt,
}

func (q *Quadrant) Valid() bool                  { return contains(quadrants, *q) }
func (q *Quadrant) String() string               { return string(*q) }
func (q *Quadrant) Value() (driver.Value, error) { return encodeEnum(q, (*Quadrant).Valid, "quadrant") }

func (q *Quadrant) Scan(src any) error { return decodeEnum(q, src, (*Quadrant).Valid, "quadrant") }

func ParseQuadrant(s string) (Quadrant, error) { return parseEnum(s, (*Quadrant).Valid, "quadrant") }
func Quadrants() []Quadrant                    { return append([]Quadrant(nil), quadrants...) }

type TaskStatus string

const (
	TaskStatusActive    TaskStatus = "ACTIVE"
	TaskStatusCompleted TaskStatus = "COMPLETED"
)

var taskStatuses = []TaskStatus{TaskStatusActive, TaskStatusCompleted}

func (s *TaskStatus) Valid() bool    { return contains(taskStatuses, *s) }
func (s *TaskStatus) String() string { return string(*s) }
func (s *TaskStatus) Value() (driver.Value, error) {
	return encodeEnum(s, (*TaskStatus).Valid, "task_status")
}

func (s *TaskStatus) Scan(src any) error {
	return decodeEnum(s, src, (*TaskStatus).Valid, "task_status")
}

func ParseTaskStatus(s string) (TaskStatus, error) {
	return parseEnum(s, (*TaskStatus).Valid, "status")
}
func TaskStatuses() []TaskStatus { return append([]TaskStatus(nil), taskStatuses...) }

type CompletionSource string

const (
	CompletionDirect        CompletionSource = "DIRECT"
	CompletionParentCascade CompletionSource = "PARENT_CASCADE"
)

var completionSources = []CompletionSource{CompletionDirect, CompletionParentCascade}

func (c *CompletionSource) Valid() bool    { return contains(completionSources, *c) }
func (c *CompletionSource) String() string { return string(*c) }
func (c *CompletionSource) Value() (driver.Value, error) {
	return encodeEnum(c, (*CompletionSource).Valid, "completion_source")
}

func (c *CompletionSource) Scan(src any) error {
	return decodeEnum(c, src, (*CompletionSource).Valid, "completion_source")
}

func CompletionSources() []CompletionSource {
	return append([]CompletionSource(nil), completionSources...)
}

type Language string

const (
	LanguageEN Language = "EN"
	LanguageRU Language = "RU"
)

var languages = []Language{LanguageEN, LanguageRU}

func (l *Language) Valid() bool                  { return contains(languages, *l) }
func (l *Language) String() string               { return string(*l) }
func (l *Language) Value() (driver.Value, error) { return encodeEnum(l, (*Language).Valid, "language") }

func (l *Language) Scan(src any) error { return decodeEnum(l, src, (*Language).Valid, "language") }

func ParseLanguage(s string) (Language, error) { return parseEnum(s, (*Language).Valid, "language") }
func Languages() []Language                    { return append([]Language(nil), languages...) }

type Plan string

const (
	PlanFree       Plan = "FREE"
	PlanPro        Plan = "PRO"
	PlanSelfHosted Plan = "SELF_HOSTED"
)

var plans = []Plan{PlanFree, PlanPro, PlanSelfHosted}

func (p *Plan) Valid() bool                  { return contains(plans, *p) }
func (p *Plan) String() string               { return string(*p) }
func (p *Plan) Value() (driver.Value, error) { return encodeEnum(p, (*Plan).Valid, "plan") }
func (p *Plan) Scan(src any) error           { return decodeEnum(p, src, (*Plan).Valid, "plan") }

func ParsePlan(s string) (Plan, error) { return parseEnum(s, (*Plan).Valid, "plan") }
func Plans() []Plan                    { return append([]Plan(nil), plans...) }

type LinkType string

const (
	LinkRelated     LinkType = "RELATED"
	LinkConnectedTo LinkType = "CONNECTED_TO"
	LinkBlocks      LinkType = "BLOCKS"
	LinkDependsOn   LinkType = "DEPENDS_ON"
)

var linkTypes = []LinkType{LinkRelated, LinkConnectedTo, LinkBlocks, LinkDependsOn}

func (l *LinkType) Valid() bool    { return contains(linkTypes, *l) }
func (l *LinkType) String() string { return string(*l) }
func (l *LinkType) Value() (driver.Value, error) {
	return encodeEnum(l, (*LinkType).Valid, "link_type")
}
func (l *LinkType) Scan(src any) error { return decodeEnum(l, src, (*LinkType).Valid, "link_type") }

// Directed reports whether the link type distinguishes source from target.
func (l *LinkType) Directed() bool { return *l == LinkBlocks || *l == LinkDependsOn }

func ParseLinkType(s string) (LinkType, error) { return parseEnum(s, (*LinkType).Valid, "type") }
func LinkTypes() []LinkType                    { return append([]LinkType(nil), linkTypes...) }

type XPSource string

const (
	XPTaskCompleted     XPSource = "TASK_COMPLETED"
	XPSubtaskCompleted  XPSource = "SUBTASK_COMPLETED"
	XPAchievementReward XPSource = "ACHIEVEMENT_REWARD"
	XPTaskReopened      XPSource = "TASK_REOPENED"
	XPTaskDeleted       XPSource = "TASK_DELETED"
	XPAdminAdjustment   XPSource = "ADMIN_ADJUSTMENT"
)

var xpSources = []XPSource{
	XPTaskCompleted, XPSubtaskCompleted, XPAchievementReward, XPTaskReopened,
	XPAdminAdjustment, XPTaskDeleted,
}

func (x *XPSource) Valid() bool    { return contains(xpSources, *x) }
func (x *XPSource) String() string { return string(*x) }
func (x *XPSource) Value() (driver.Value, error) {
	return encodeEnum(x, (*XPSource).Valid, "xp_source")
}
func (x *XPSource) Scan(src any) error { return decodeEnum(x, src, (*XPSource).Valid, "xp_source") }

func XPSources() []XPSource { return append([]XPSource(nil), xpSources...) }

type ActivityEventType string

const (
	EventTaskCreated         ActivityEventType = "TASK_CREATED"
	EventTaskCompleted       ActivityEventType = "TASK_COMPLETED"
	EventSubtaskCompleted    ActivityEventType = "SUBTASK_COMPLETED"
	EventTaskLinked          ActivityEventType = "TASK_LINKED"
	EventLevelUp             ActivityEventType = "LEVEL_UP"
	EventAchievementUnlocked ActivityEventType = "ACHIEVEMENT_UNLOCKED"
	EventStreakExtended      ActivityEventType = "STREAK_EXTENDED"
	EventGraphOpened         ActivityEventType = "GRAPH_OPENED"
)

var activityEventTypes = []ActivityEventType{
	EventTaskCreated, EventTaskCompleted, EventSubtaskCompleted, EventTaskLinked,
	EventLevelUp, EventAchievementUnlocked, EventStreakExtended, EventGraphOpened,
}

func (a *ActivityEventType) Valid() bool    { return contains(activityEventTypes, *a) }
func (a *ActivityEventType) String() string { return string(*a) }
func (a *ActivityEventType) Value() (driver.Value, error) {
	return encodeEnum(a, (*ActivityEventType).Valid, "activity_event_type")
}

func (a *ActivityEventType) Scan(src any) error {
	return decodeEnum(a, src, (*ActivityEventType).Valid, "activity_event_type")
}

func ActivityEventTypes() []ActivityEventType {
	return append([]ActivityEventType(nil), activityEventTypes...)
}

type AchievementCategory string

const (
	CategoryTasks       AchievementCategory = "TASKS"
	CategoryXP          AchievementCategory = "XP"
	CategoryLevel       AchievementCategory = "LEVEL"
	CategoryStreak      AchievementCategory = "STREAK"
	CategoryLinks       AchievementCategory = "LINKS"
	CategoryExploration AchievementCategory = "EXPLORATION"
)

var achievementCategories = []AchievementCategory{
	CategoryTasks, CategoryXP, CategoryLevel, CategoryStreak, CategoryLinks, CategoryExploration,
}

func (a *AchievementCategory) Valid() bool    { return contains(achievementCategories, *a) }
func (a *AchievementCategory) String() string { return string(*a) }
func (a *AchievementCategory) Value() (driver.Value, error) {
	return encodeEnum(a, (*AchievementCategory).Valid, "achievement_category")
}

func (a *AchievementCategory) Scan(src any) error {
	return decodeEnum(a, src, (*AchievementCategory).Valid, "achievement_category")
}

func AchievementCategories() []AchievementCategory {
	return append([]AchievementCategory(nil), achievementCategories...)
}

type NotificationType string

const (
	NotifyDeadlineApproaching NotificationType = "DEADLINE_APPROACHING"
	NotifyTaskOverdue         NotificationType = "TASK_OVERDUE"
	NotifyAchievement         NotificationType = "ACHIEVEMENT_UNLOCKED"
	NotifyLevelUp             NotificationType = "LEVEL_UP"
	NotifyStreakExtended      NotificationType = "STREAK_EXTENDED"
)

var notificationTypes = []NotificationType{
	NotifyDeadlineApproaching, NotifyTaskOverdue, NotifyAchievement, NotifyLevelUp,
	NotifyStreakExtended,
}

func (n *NotificationType) Valid() bool    { return contains(notificationTypes, *n) }
func (n *NotificationType) String() string { return string(*n) }
func (n *NotificationType) Value() (driver.Value, error) {
	return encodeEnum(n, (*NotificationType).Valid, "notification_type")
}

func (n *NotificationType) Scan(src any) error {
	return decodeEnum(n, src, (*NotificationType).Valid, "notification_type")
}

func NotificationTypes() []NotificationType {
	return append([]NotificationType(nil), notificationTypes...)
}

// TaskColor is presentation metadata, stored as text rather than as an enum.
type TaskColor string

const (
	ColorNone   TaskColor = "NONE"
	ColorCyan   TaskColor = "CYAN"
	ColorViolet TaskColor = "VIOLET"
	ColorAmber  TaskColor = "AMBER"
	ColorRose   TaskColor = "ROSE"
	ColorTeal   TaskColor = "TEAL"
	ColorSlate  TaskColor = "SLATE"
)

var taskColors = []TaskColor{
	ColorNone, ColorCyan, ColorViolet, ColorAmber, ColorRose, ColorTeal, ColorSlate,
}

func (c *TaskColor) Valid() bool                  { return contains(taskColors, *c) }
func (c *TaskColor) String() string               { return string(*c) }
func (c *TaskColor) Value() (driver.Value, error) { return encodeEnum(c, (*TaskColor).Valid, "color") }

func (c *TaskColor) Scan(src any) error { return decodeEnum(c, src, (*TaskColor).Valid, "color") }

func ParseTaskColor(s string) (TaskColor, error) { return parseEnum(s, (*TaskColor).Valid, "color") }
func TaskColors() []TaskColor                    { return append([]TaskColor(nil), taskColors...) }

func PGEnumTypes() map[string][]string {
	return map[string][]string{
		"quadrant_enum":             codes(quadrants),
		"task_status_enum":          codes(taskStatuses),
		"completion_source_enum":    codes(completionSources),
		"language_enum":             codes(languages),
		"plan_enum":                 codes(plans),
		"link_type_enum":            codes(linkTypes),
		"xp_source_enum":            codes(xpSources),
		"activity_event_type_enum":  codes(activityEventTypes),
		"achievement_category_enum": codes(achievementCategories),
		"notification_type_enum":    codes(notificationTypes),
	}
}
