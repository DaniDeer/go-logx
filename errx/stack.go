package errx

func (e *Error) StackTrace() []StackFrame {
	return e.stack
}

func stackAttrs(stack []StackFrame) any {
	out := make([]map[string]any, 0, len(stack))

	for _, frame := range stack {
		out = append(out, map[string]any{
			"function": frame.Function,
			"file":     frame.File,
			"line":     frame.Line,
		})
	}

	return out
}