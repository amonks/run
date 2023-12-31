// Code generated by "stringer -type TaskStatus"; DO NOT EDIT.

package run

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[taskStatusInvalid-0]
	_ = x[TaskStatusNotStarted-1]
	_ = x[TaskStatusRunning-2]
	_ = x[TaskStatusRestarting-3]
	_ = x[TaskStatusFailed-4]
	_ = x[TaskStatusDone-5]
}

const _TaskStatus_name = "taskStatusInvalidTaskStatusNotStartedTaskStatusRunningTaskStatusRestartingTaskStatusFailedTaskStatusDone"

var _TaskStatus_index = [...]uint8{0, 17, 37, 54, 74, 90, 104}

func (i TaskStatus) String() string {
	if i < 0 || i >= TaskStatus(len(_TaskStatus_index)-1) {
		return "TaskStatus(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _TaskStatus_name[_TaskStatus_index[i]:_TaskStatus_index[i+1]]
}
