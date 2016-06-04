package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
)

var (
	watchDir    string
	recursive   bool
	exclude     string
	cmdName     string
	waitForExit bool
	initCmd     bool
)

func init() {
	flag.StringVar(&watchDir, "watch", "./", "directory to watch")
	flag.BoolVar(&recursive, "r", true, "watch directory recursively")
	flag.StringVar(&exclude, "exclude", ".git", "directory name(s) to exclude (comma separated)")

	flag.StringVar(&cmdName, "cmd", "", "command to run when an event occurs")
	flag.BoolVar(&waitForExit, "wait", false, "wait for command to finish running")
	flag.BoolVar(&initCmd, "init", true, "execute command immidiately (does not wait for first change event)")
}

var (
	magenta = color.New(color.FgCyan)
	red     = color.New(color.FgRed)
	yellow  = color.New(color.FgYellow)
)

func main() {
	flag.Parse()

	var cmd *exec.Cmd

	if cmdName == "" {
		log.Fatalln("Please specify a command to run. For more info see usage.")
	}

	if watchDir == "" {
		log.Fatalln("Please specify a directory to watch. For more info see usage.")
	}

	if _, err := os.Stat(watchDir); err != nil {
		log.Fatalln("Error:", err)
	}

	excludedDirs := strings.Split(exclude, ",")

	startCmd := func() *exec.Cmd {
		magenta.Println("Starting...")
		cmd = exec.Command("bash", "-c", cmdName)
		// set new process group for process; simplifies cancelling process tree
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stdout

		err := cmd.Start()
		if err != nil {
			log.Fatalln(err)
		}
		return cmd
	}

	stopCmd := func() {
		// if waitForExit flag is set, just wait till cmd completes
		if waitForExit {
			magenta.Println("Waiting for task to complete.")
			err := cmd.Wait()
			if err != nil {
				red.Println("Wait err:", err)
			}
		} else {
			magenta.Println("Stopping task...")
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
				syscall.Kill(-pgid, syscall.SIGTERM)
			}
			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()
			select {
			case <-time.After(2 * time.Second):
				// sigint failed, so KILL it
				if err := cmd.Process.Kill(); err != nil {
					log.Fatalln("Failed to KILL task:", err)
				}
				magenta.Println("KILLed process")
			case err := <-done:
				if err != nil {
					red.Println("Stopped. Status:", err)
				} else {
					magenta.Println("Task stopped.")
				}
			}
		}
	}

	// if initCmd flag is set (set by default), start command before watchers
	if initCmd {
		cmd = startCmd()
	}

	// Since we're creating a new process group for the task, we need a way of
	// cleaning it up when the watcher exits, so we watch for signals to do so
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for _ = range sigchan {
			stopCmd()
			os.Exit(0)
		}
	}()

	notifyChan := make(chan struct{})
	go Watch(watchDir, notifyChan, recursive, excludedDirs)

	for {
		_ = <-notifyChan
		stopCmd()
		cmd = startCmd()
	}
}

func watch(root string, watcher *fsnotify.Watcher, rec bool, excludes []string) error {
	if !rec {
		return watcher.Add(root)
	}
	err := filepath.Walk(root, func(path string, i os.FileInfo, err error) error {
		if i.IsDir() {
			for _, excl := range excludes {
				if i.Name() == excl {
					return filepath.SkipDir
				}
			}
			watcher.Add(path)
		}
		return nil
	})
	return err
}

// Watch watches given root diretory and sends a notification on a WRITE event
func Watch(root string, notify chan struct{}, recursive bool, excludes []string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		red.Println("Watcher:", err)
		return
	}
	defer watcher.Close()

	err = watch(root, watcher, recursive, excludes)
	if err != nil {
		red.Println("Watcher:", err)
		return
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				yellow.Println("Event:", event)
				notify <- struct{}{}
			}
		case err := <-watcher.Errors:
			red.Println("Watcher:", err)
		}
	}
}
