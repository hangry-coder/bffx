package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/hangry-coder/bffx/pkg/app"
	"github.com/hangry-coder/bffx/pkg/compiler"
	"github.com/hangry-coder/bffx/pkg/dev"
	"github.com/hangry-coder/bffx/pkg/docsbundle"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/fsnotify/fsnotify"
)

func HandleDev(args []string) {
	watch := false
	root := "."

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-h" || arg == "--help" || arg == "help" {
			fmt.Println("Usage: bffx dev [--root DIR] [--watch]")
			fmt.Println("  Run compiler sync, build ./cmd/orchestrator, and start the API server.")
			fmt.Println("  --watch: Monitor manifests, hooks, and cmd/ for changes and restart.")
			return
		}
		if arg == "--watch" {
			watch = true
		}
		if arg == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		log.Fatalf("invalid --root %q: %v", root, err)
	}
	root = rootAbs

	if err := app.BootstrapEnv(root); err != nil {
		log.Printf("[bffx] Warning: failed to bootstrap env: %v", err)
	}

	if err := docsbundle.EnsureBundled(root); err != nil {
		log.Printf("[bffx] Warning: could not bundle admin guide docs: %v", err)
	}

	if !watch {
		if err := runOrchestrator(root); err != nil {
			log.Fatalf("dev server failed: %v", err)
		}
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	roots := dev.DevWatchRoots(root)
	if err := dev.AddRecursiveWatch(watcher, roots); err != nil {
		log.Fatalf("watch setup failed: %v", err)
	}
	logger.Info("Watching %v for changes...", roots)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var (
		cmd      *exec.Cmd
		mu       sync.Mutex
		restart  = make(chan bool, 1)
		timer    *time.Timer
		debounce = 500 * time.Millisecond
	)

	restart <- true

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if dev.ShouldRestart(event.Name, event.Op) {
					mu.Lock()
					if timer != nil {
						timer.Stop()
					}
					timer = time.AfterFunc(debounce, func() {
						select {
						case restart <- true:
						default:
						}
					})
					mu.Unlock()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error("Watcher error: %v", err)
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-restart:
			if cmd != nil && cmd.Process != nil {
				logger.Info("Detected changes, restarting...")
				_ = cmd.Process.Kill()
				_, _ = cmd.Process.Wait()
			}

			logger.Info("Syncing manifests...")
			if _, err := compiler.Sync(root, false); err != nil {
				logger.Error("Sync failed: %v (retrying on next change)", err)
				continue
			}

			logger.Info("Building orchestrator...")
			entry := "./cmd/orchestrator"
			if _, err := os.Stat(filepath.Join(root, "cmd", "api")); err == nil {
				entry = "./cmd/api"
			}
			buildCmd := exec.Command("go", "build", "-o", ".bffx/orchestrator", entry)
			buildCmd.Dir = root
			buildCmd.Env = compiler.GoBuildEnv()
			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr
			if err := buildCmd.Run(); err != nil {
				logger.Error("Build failed: %v (retrying on next change)", err)
				continue
			}

			logger.Info("Starting dev server...")
			cmd = exec.Command("./.bffx/orchestrator")
			cmd.Dir = root
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Env = os.Environ()
			if err := cmd.Start(); err != nil {
				logger.Error("Failed to start orchestrator: %v", err)
			}

		case sig := <-sigChan:
			logger.Info("Received signal %v, shutting down...", sig)
			cancel()
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
				_, _ = cmd.Process.Wait()
			}
			return
		}
	}
}

func runOrchestrator(root string) error {
	logger.Info("Starting bffx dev server for %s", root)
	if _, err := compiler.Sync(root, false); err != nil {
		return fmt.Errorf("sync failed: %v", err)
	}

	entry := "./cmd/orchestrator"
	if _, err := os.Stat(filepath.Join(root, "cmd", "api")); err == nil {
		entry = "./cmd/api"
	}

	buildCmd := exec.Command("go", "build", "-o", ".bffx/orchestrator", entry)
	buildCmd.Dir = root
	buildCmd.Env = compiler.GoBuildEnv()
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build orchestrator: %v", err)
	}

	cmd := exec.Command("./.bffx/orchestrator")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return cmd.Run()
}
