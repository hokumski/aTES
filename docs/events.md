# Services

- Auth service
- TaskManager service
- Accounting service
- Payment service

- Each service sends events to broker.
- Each service makes sync requests to Auth service.
- Each service implements its own log.

# aTES events

### UserLogin
- produced by Auth (internal event, log only)

### UserLogout
- produced by Auth (internal event, log only)

### UserCreated
- produced by Auth
- consumed by TaskManager, Accounting (to create new account representation)

### TaskCreated
- produced by TaskManager
- consumed by Accounting

Important: Task is being created "incomplete" (Status=NEW) in TaskManager service. 

Then, TaskCreated event is processed by Accounting service, and after all necessary information (costs and assignment) 
is set, TaskAssigned event is produced.

### TaskAssigned
- produced by Accounting
- consumed by TaskManager, Accounting (internally)

TaskAssigned event is consumed by 2 services:
- by Accounting service to deduct cost from the assigned user, 
- by TaskManager service to change Status=OPEN. Only opened tasks (not new) are listed with GetMyTasks command.

### TaskCompleted
- produced by TaskManager
- consumed by Accounting

Status is set to COMPLETED 

### EndOfDay
- produced by Accounting (cron job)
- consumed by Accounting (internal event, leads to N * PayoutUser events)

### PayoutUser
- produced by Accounting (internal event)
