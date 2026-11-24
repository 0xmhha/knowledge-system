// Command tracer is the prototype of the message tracing rules
// (docs/experiments/log-backtrace/RULES.md). It starts from a failure message,
// finds the code that emits it and follows conditions, fields, returned values
// and callers to a ranked list of declarations, each with the way it was
// reached. It reads one graph and the source tree of the same commit and never
// writes to either.
//
//	tracer -db graph.db -src <source tree> -message "<text>" [-top 20]
package main
