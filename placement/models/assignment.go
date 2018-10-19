package models

// Assignment represents the assignment of a task to a host.
// One host can be used in multiple assignments.
type Assignment struct {
	HostOffers *HostOffers `json:"host"`
	Task       *Task       `json:"task"`
	Reason     string
}

// GetHost returns the host that the task was assigned to.
func (a *Assignment) GetHost() *HostOffers {
	return a.HostOffers
}

// SetHost sets the host in the assignment to the given host.
func (a *Assignment) SetHost(host *HostOffers) {
	a.HostOffers = host
}

// GetTask returns the task of the assignment.
func (a *Assignment) GetTask() *Task {
	return a.Task
}

// SetTask sets the task in the assignment to the given task.
func (a *Assignment) SetTask(task *Task) {
	a.Task = task
}

// GetReason returns the reason why the assignment was unsuccessful
func (a *Assignment) GetReason() string {
	return a.Reason
}

// SetReason sets the reason for the failed assignment
func (a *Assignment) SetReason(reason string) {
	a.Reason = reason
}

// NewAssignment will create a new empty assignment from a task.
func NewAssignment(task *Task) *Assignment {
	return &Assignment{
		Task: task,
	}
}
