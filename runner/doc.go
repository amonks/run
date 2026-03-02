// Package runner executes a collection of tasks with dependency management,
// file watching, and process lifecycle control.
//
// # Conceptual Overview
//
// 1. You call [taskfile.Load] to parse task files and get a [task.Library].
//    - You can also generate your own task set, or append your own tasks onto it.
//    - Tasks just implement an interface.
// 2. You combine a [RunType], a task library, an ID, and a [MultiWriter] to
//    get a [Run] using [New].
// 3. You call [Run.Start] which blocks until the run completes or is canceled.
//    - You can also make your own [MultiWriter].
package runner
