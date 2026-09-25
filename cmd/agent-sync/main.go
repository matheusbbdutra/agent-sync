package main

import (
	"fmt"
	"os"

	"github.com/matheusdutra/agent-sync/internal/apply"
	"github.com/matheusdutra/agent-sync/internal/budget"
	"github.com/matheusdutra/agent-sync/internal/doctor"
	"github.com/matheusdutra/agent-sync/internal/event"
	"github.com/matheusdutra/agent-sync/internal/memory"
	"github.com/matheusdutra/agent-sync/internal/skills"
	"github.com/matheusdutra/agent-sync/internal/state"
)

func main() {
	if len(os.Args) >= 2 {
		var err error
		switch os.Args[1] {
		case "state":
			err = state.RunCommand(os.Args[2:])
		case "event":
			err = event.RunCommand(os.Args[2:])
		case "memory":
			err = memory.RunCommand(os.Args[2:])
		case "budget":
			err = budget.RunCommand(os.Args[2:])
		case "skills":
			err = skills.RunCommand(os.Args[2:])
		case "doctor":
			err = doctor.RunCommand(os.Args[2:])
		default:
			err = apply.RunCommand(os.Args[1:])
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := apply.RunCommand(nil); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}
