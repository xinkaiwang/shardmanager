package klogging

import (
	"context"
	"log/slog"
)

// Fatal logs at LevelFatal and terminates the process with exit code 1
// (KLOG-003). Use it for unrecoverable states — config errors, broken
// invariants — where continuing does more damage than dying (same philosophy
// as kmetrics' tag-conflict exit).
//
// In production this really exits; tests swap MockOsProvider via os.go.
//
// PRECONDITION (do not break): the fatal line must reach the output before
// OsExit runs. This holds because Handler.Handle writes synchronously to
// Output. If Output is ever wrapped in a buffered/async writer, this final
// log — the most important one — would be silently lost. TestFatal_LogsBeforeExit
// pins this behavior.
//
// os.Exit runs no deferred functions: spans aren't ended, servers aren't
// drained. That is inherent to fatal semantics.
func Fatal(ctx context.Context, msg string, attrs ...slog.Attr) {
	slog.Default().LogAttrs(ctx, LevelFatal, msg, attrs...)
	OsExit(1)
}
