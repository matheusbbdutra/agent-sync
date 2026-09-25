package event

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// RunCommand implementa `agent-sync event <subcommand>`.
func RunCommand(args []string) error {
	if len(args) == 0 {
		return eventUsage(os.Stderr)
	}
	switch args[0] {
	case "read":
		return runEventRead(args[1:])
	case "stats":
		return runEventStats(args[1:])
	case "tail":
		return runEventTail(args[1:])
	case "help", "-h", "--help":
		return eventUsage(os.Stdout)
	default:
		return fmt.Errorf("event: subcommand desconhecido: %q", args[0])
	}
}

func eventUsage(w io.Writer) error {
	fmt.Fprintf(w, "Uso: agent-sync event <subcommand> [-root <path>]\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  read          Imprime eventos do log (JSONL por linha, ordenados por ts)\n")
	fmt.Fprintf(w, "  stats         Conta eventos por kind/actor; saida JSON estruturada\n")
	fmt.Fprintf(w, "  tail          Stream live de eventos (poll 1s; util em dev)\n")
	fmt.Fprintf(w, "\nFlags (read/stats):\n")
	fmt.Fprintf(w, "  -last N       Apenas os ultimos N eventos\n")
	fmt.Fprintf(w, "  -kind K       Filtra por kind (decision|action|blocker|open_question|state_render)\n")
	fmt.Fprintf(w, "  -since RFC    Apenas eventos com ts >= valor (RFC3339)\n")
	fmt.Fprintf(w, "  -root <path>  Project root (default: derivado do binario + AGENT_SYNC_HOME + cwd)\n")
	return nil
}

type eventFlags struct {
	fs    *flag.FlagSet
	root  string
	last  int
	kind  string
	since string
}

func newEventFlags(name string) *eventFlags {
	f := &eventFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.StringVar(&f.root, "root", "", "Project root")
	f.fs.IntVar(&f.last, "last", 0, "Ultimos N eventos (0 = todos)")
	f.fs.StringVar(&f.kind, "kind", "", "Filtra por kind")
	f.fs.StringVar(&f.since, "since", "", "Filtra por ts >= RFC3339")
	return f
}

func (f *eventFlags) parse(args []string) error {
	return f.fs.Parse(args)
}

func (f *eventFlags) readOpts() (EventReadOptions, error) {
	opts := EventReadOptions{Last: f.last, Kind: f.kind}
	if f.since != "" {
		ts, err := time.Parse(time.RFC3339, f.since)
		if err != nil {
			return opts, fmt.Errorf("since invalido (esperado RFC3339): %w", err)
		}
		opts.Since = ts
	}
	return opts, nil
}

func runEventRead(args []string) error {
	f := newEventFlags("agent-sync event read")
	if err := f.parse(args); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	opts, err := f.readOpts()
	if err != nil {
		return err
	}
	events, err := ReadEvents(root, opts)
	if err != nil {
		return err
	}
	for _, e := range events {
		line, err := json.Marshal(&e)
		if err != nil {
			return err
		}
		fmt.Println(string(line))
	}
	return nil
}

func runEventStats(args []string) error {
	f := newEventFlags("agent-sync event stats")
	if err := f.parse(args); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	opts, err := f.readOpts()
	if err != nil {
		return err
	}
	stats, err := StatsEvents(root, opts)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(&stats, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func runEventTail(args []string) error {
	f := newEventFlags("agent-sync event tail")
	if err := f.parse(args); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	if err := pathutil.EnsureStateDir(root); err != nil {
		return err
	}
	path := filepath.Join(root, pathutil.SessionStateDirName, SessionEventFileName)
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		f, _ := os.Create(path)
		if f != nil {
			f.Close()
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	buf := make([]byte, 64*1024)
	var pending strings.Builder
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		n, err := file.Read(buf)
		if n > 0 {
			pending.Write(buf[:n])
			for {
				line, rest, ok := splitLine(pending.String())
				if !ok {
					break
				}
				fmt.Println(line)
				pending.Reset()
				pending.WriteString(rest)
			}
		}
		if err != nil && err != io.EOF {
			return err
		}
		<-ticker.C
	}
}

func splitLine(s string) (line, rest string, ok bool) {
	idx := strings.IndexByte(s, '\n')
	if idx < 0 {
		return "", s, false
	}
	return s[:idx], s[idx+1:], true
}
