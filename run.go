// Run runs a collection of programs specified in tasks.toml files, and
// provides a UI for inspecting their execution. Run displays long-lived
// processes in an interactive TUI. Run also works well for short-lived
// processes, and its interleaved output can be sent to a file.
//
// Run can be used and extended programatically through its Go API, which is
// documented here. Run's primary documentation is on [Github].
//
// [Github]: https://github.com/amonks/run
//
// # Conceptual Overview
//
// 1. You call Load to parse task files and get a Task set.
//    - You can also generate your own task set, or append your own tasks onto it.
//    - Tasks just implement an interface.
// 2. You combine a task list with an ID to get a Run.
//    - You can also generate your own Run.
//    - Runs just implement an interface.
// 3. You pass a UI into a Run and Start it.
//    - You can also make your own UI.
//    - You can also use a Run UI with any other collection of processes
//      that expect io.Writers.
package run
