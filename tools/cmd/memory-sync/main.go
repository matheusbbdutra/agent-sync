// Command memory-sync sincroniza memórias persistentes entre SQLite local e Turso Cloud.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/matheusdutra/token-tools/internal/agentmemory"
)

func run(args []string) error {
	flags := flag.NewFlagSet("memory-sync", flag.ContinueOnError)
	phase := flags.String("phase", "", "start, end, resolve-local ou resolve-remote")
	conflict := flags.String("conflict", "", "identificador anônimo de conflito")
	timeout := flags.Duration("timeout", 30*time.Second, "tempo máximo da sincronização")
	initConfig := flags.Bool("init", false, "cria a configuração local e mostra o caminho")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *timeout <= 0 {
		return fmt.Errorf("use -init ou -phase start, end, resolve-local ou resolve-remote com timeout positivo")
	}
	if *initConfig {
		if *phase != "" {
			return fmt.Errorf("-init e -phase não podem ser usados juntos")
		}
		path, err := agentmemory.EnsureConfig()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	}
	if *phase != "start" && *phase != "end" && *phase != "resolve-local" && *phase != "resolve-remote" {
		return fmt.Errorf("use -init ou -phase start, end, resolve-local ou resolve-remote")
	}
	address, token, err := agentmemory.LoadRemoteConfig()
	if err != nil {
		return err
	}
	path, err := agentmemory.DefaultDBPath()
	if err != nil {
		return err
	}
	store, err := agentmemory.Open(path)
	if err != nil {
		return err
	}
	defer store.Close()
	remote, err := agentmemory.OpenRemote(address, token)
	if err != nil {
		return err
	}
	defer remote.Close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if *phase == "resolve-local" || *phase == "resolve-remote" {
		choice := "local"
		if *phase == "resolve-remote" {
			choice = "remote"
		}
		if err := store.Resolve(ctx, remote, *conflict, choice); err != nil {
			return err
		}
		fmt.Println("memory-sync: conflito resolvido")
		return nil
	}
	var count int
	if *phase == "start" {
		count, err = store.Pull(ctx, remote)
	} else {
		count, err = store.Push(ctx, remote)
	}
	if err != nil {
		return err
	}
	fmt.Printf("memory-sync: %d memórias sincronizadas (%s)\n", count, *phase)
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "memory-sync:", err)
		os.Exit(1)
	}
}
