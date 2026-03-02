// Package runner executes a collection of tasks with dependency management,
// file watching, and process lifecycle control.
//
// # Conceptual Overview
//
// 1. You call [taskfile.Load] to parse task files and get a [task.Library].
//    - You can also generate your own task set, or append your own tasks onto it.
//    - Tasks just implement an interface.
// 2. You combine a task library with an ID to get a [Run] using [New].
// 3. You pass a [UI] into a Run and Start it.
//    - You can also make your own UI.
//    - You can also use a Run UI with any other collection of processes
//      that expect io.Writers.
package runner
