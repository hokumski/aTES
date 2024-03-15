package schema

// UserRole copies values from Auth.Role
type UserRole int

const (
	RoleAdmin UserRole = iota + 1
	RoleUser
	RoleManager
	RoleAccountant
)

// TaskStatus copies values from TaskManager.Status
type TaskStatus int

const (
	StatusOpen TaskStatus = iota + 1
	StatusCompleted
)

// AccountOperationType copies values from Accounting.OperationType
type AccountOperationType int

const (
	CostOfAssignment AccountOperationType = iota + 1
	CompletionReward
	WagePayment
)
