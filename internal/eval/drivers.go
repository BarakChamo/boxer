package eval

// Drivers lists every harness and orchestrator driver in report order.
func Drivers() []Driver {
	return []Driver{
		Claude{},
		Codex{},
		Gemini{},
		OpenCode{},
		Pi{},
		Kimi{},
		Grok{},
		Inside{},
		InsideACP{},
		OpenHands{},
		Paperclip,
		T3,
		Multica,
	}
}
